package sandbox

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"starclaw.net/spore/pkg/manifest"
)

func newPlatformRunner() Runner {
	return &windowsRunner{}
}

func platformInstallHint() string {
	return "Windows Job Objects are built-in, no additional install needed"
}

// windowsRunner uses Windows Job Objects for resource limits.
// Filesystem isolation is limited on Windows without containers,
// so we focus on CPU/memory limits and process group management.
type windowsRunner struct{}

func (r *windowsRunner) Name() string { return "job-object" }

func (r *windowsRunner) Available() bool {
	// Job Objects are always available on Windows
	return true
}

// Wrap applies resource limits via a PowerShell wrapper that creates a Job Object.
// On Windows, true filesystem isolation requires containers (Hyper-V/WSL),
// so we provide resource limits + process group management instead.
func (r *windowsRunner) Wrap(cmd *exec.Cmd, binPath, dataDir, logDir string, cfg manifest.Sandbox) error {
	if cfg.MaxMemoryMB == 0 && cfg.MaxCPU == 0 {
		// No resource limits requested — nothing to wrap
		return nil
	}

	// Use a PowerShell script to create a Job Object with limits, then launch the process inside it.
	// This ensures the process (and all children) respect the resource limits.
	var psLines []string
	psLines = append(psLines, `$ErrorActionPreference = 'Stop'`)

	// Create Job Object with limits
	psLines = append(psLines, `Add-Type -TypeDefinition @"
using System;
using System.Runtime.InteropServices;

public class JobObject {
    [DllImport("kernel32.dll", CharSet = CharSet.Unicode)]
    public static extern IntPtr CreateJobObject(IntPtr lpJobAttributes, string lpName);

    [DllImport("kernel32.dll")]
    public static extern bool SetInformationJobObject(IntPtr hJob, int infoType, IntPtr lpJobInfo, uint cbInfoLength);

    [DllImport("kernel32.dll")]
    public static extern bool AssignProcessToJobObject(IntPtr hJob, IntPtr hProcess);

    [StructLayout(LayoutKind.Sequential)]
    public struct JOBOBJECT_BASIC_LIMIT_INFORMATION {
        public long PerProcessUserTimeLimit;
        public long PerJobUserTimeLimit;
        public uint LimitFlags;
        public UIntPtr MinimumWorkingSetSize;
        public UIntPtr MaximumWorkingSetSize;
        public uint ActiveProcessLimit;
        public UIntPtr Affinity;
        public uint PriorityClass;
        public uint SchedulingClass;
    }

    [StructLayout(LayoutKind.Sequential)]
    public struct IO_COUNTERS {
        public ulong ReadOperationCount, WriteOperationCount, OtherOperationCount;
        public ulong ReadTransferCount, WriteTransferCount, OtherTransferCount;
    }

    [StructLayout(LayoutKind.Sequential)]
    public struct JOBOBJECT_EXTENDED_LIMIT_INFORMATION {
        public JOBOBJECT_BASIC_LIMIT_INFORMATION BasicLimitInformation;
        public IO_COUNTERS IoInfo;
        public UIntPtr ProcessMemoryLimit;
        public UIntPtr JobMemoryLimit;
        public UIntPtr PeakProcessMemoryLimit;
        public UIntPtr PeakJobMemoryLimit;
    }
}
"@`)

	psLines = append(psLines, `$job = [JobObject]::CreateJobObject([IntPtr]::Zero, $null)`)

	// Build limit flags
	flags := []string{}
	if cfg.MaxMemoryMB > 0 {
		flags = append(flags, "0x0100") // JOB_OBJECT_LIMIT_PROCESS_MEMORY
	}
	if cfg.MaxCPU > 0 {
		flags = append(flags, "0x0004") // JOB_OBJECT_LIMIT_JOB_TIME — approximate via affinity
	}

	if len(flags) > 0 {
		psLines = append(psLines, `$info = New-Object JobObject+JOBOBJECT_EXTENDED_LIMIT_INFORMATION`)
		psLines = append(psLines, fmt.Sprintf(`$info.BasicLimitInformation.LimitFlags = %s`, strings.Join(flags, " -bor ")))
		if cfg.MaxMemoryMB > 0 {
			psLines = append(psLines, fmt.Sprintf(`$info.ProcessMemoryLimit = [UIntPtr]::new(%d)`, cfg.MaxMemoryMB*1024*1024))
		}
		psLines = append(psLines, `$size = [System.Runtime.InteropServices.Marshal]::SizeOf($info)`)
		psLines = append(psLines, `$ptr = [System.Runtime.InteropServices.Marshal]::AllocHGlobal($size)`)
		psLines = append(psLines, `[System.Runtime.InteropServices.Marshal]::StructureToPtr($info, $ptr, $false)`)
		psLines = append(psLines, `[JobObject]::SetInformationJobObject($job, 9, $ptr, [uint]$size) | Out-Null`) // 9 = JobObjectExtendedLimitInformation
		psLines = append(psLines, `[System.Runtime.InteropServices.Marshal]::FreeHGlobal($ptr)`)
	}

	// Start the process and assign to job
	origArgs := strings.Join(cmd.Args[1:], " ")
	psLines = append(psLines, fmt.Sprintf(`$proc = Start-Process -FilePath '%s' -ArgumentList '%s' -WorkingDirectory '%s' -PassThru -NoNewWindow`,
		binPath, origArgs, cmd.Dir))
	psLines = append(psLines, `[JobObject]::AssignProcessToJobObject($job, $proc.Handle) | Out-Null`)
	psLines = append(psLines, `$proc.Id`) // output PID

	script := strings.Join(psLines, "\n")

	// Write script to temp file
	tmpFile := fmt.Sprintf("%s\\spore-sandbox-%d.ps1", os.TempDir(), os.Getpid())
	os.WriteFile(tmpFile, []byte(script), 0644)

	cmd.Path, _ = exec.LookPath("powershell")
	cmd.Args = []string{"powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", tmpFile}

	return nil
}

// ApplyPostStart applies resource limits after the process has started (alternative approach).
// Uses `wmic` or PowerShell to set process priority and affinity.
func ApplyPostStart(pid int, cfg manifest.Sandbox) error {
	if cfg.MaxCPU > 0 && cfg.MaxCPU < 1.0 {
		// Set process priority to below-normal to limit CPU impact
		exec.Command("wmic", "process", "where",
			fmt.Sprintf("ProcessId=%d", pid),
			"CALL", "SetPriority", "16384").Run() // BELOW_NORMAL_PRIORITY_CLASS
	}
	return nil
}

// unused but keeps compiler happy for strconv import
var _ = strconv.Itoa
