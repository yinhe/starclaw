import { Monitor, Terminal, Apple, Download, ExternalLink } from 'lucide-react';

const STARAI_BASE = 'https://star-ai.net/downloads';
const NYDUS_BASE = 'https://nydus.starclaw.net/spore/releases';

const PACKAGES = [
  {
    platform: 'Windows',
    icon: Monitor,
    mirrorUrl: `${STARAI_BASE}/StarClaw-Setup.exe`,
    fallbackUrl: `${NYDUS_BASE}/StarClaw-Setup.exe`,
    size: '~22 MB',
    arch: 'x86_64',
    filename: 'StarClaw-Setup.exe',
    note: '下载后双击运行，支持选择安装目录',
  },
  {
    platform: 'Linux',
    icon: Terminal,
    mirrorUrl: `${STARAI_BASE}/StarClaw-Setup-linux-amd64`,
    fallbackUrl: `${NYDUS_BASE}/StarClaw-Setup-linux-amd64`,
    size: '~22 MB',
    arch: 'x86_64',
    filename: 'StarClaw-Setup-linux-amd64',
    note: '下载后 chmod +x 然后运行',
  },
  {
    platform: 'macOS',
    icon: Apple,
    mirrorUrl: `${STARAI_BASE}/StarClaw-Setup-darwin-arm64`,
    fallbackUrl: `${NYDUS_BASE}/StarClaw-Setup-darwin-arm64`,
    size: '~22 MB',
    arch: 'Apple Silicon',
    filename: 'StarClaw-Setup-darwin-arm64',
    note: '下载后 chmod +x 然后运行',
  },
];

export default function DownloadPage() {
  return (
    <div className="space-y-8">
      <div>
        <h1 className="text-2xl font-bold text-white">下载客户端</h1>
        <p className="text-gray-400 text-sm mt-1">安装 StarClaw 客户端，在本地运行 AI Agent</p>
      </div>

      <div className="grid gap-4">
        {PACKAGES.map((pkg) => {
          const Icon = pkg.icon;
          return (
            <div key={pkg.platform} className="bg-gray-900 border border-gray-800 rounded-xl p-5 hover:border-amber-500/30 transition-colors">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-4">
                  <div className="w-12 h-12 rounded-xl bg-gray-800 flex items-center justify-center">
                    <Icon className="w-6 h-6 text-amber-400" />
                  </div>
                  <div>
                    <div className="font-semibold text-white text-lg">{pkg.platform}</div>
                    <div className="text-xs text-gray-500 mt-0.5">{pkg.arch} · {pkg.size} · {pkg.filename}</div>
                    <div className="text-xs text-gray-400 mt-1">{pkg.note}</div>
                  </div>
                </div>
                <div className="flex items-center gap-2">
                  <a
                    href={pkg.mirrorUrl}
                    className="flex items-center gap-1.5 px-4 py-2 bg-amber-500 hover:bg-amber-400 text-black font-medium rounded-lg text-sm transition-colors"
                  >
                    <Download className="w-4 h-4" /> 下载
                  </a>
                  <a
                    href={pkg.fallbackUrl}
                    className="flex items-center gap-1.5 px-3 py-2 border border-gray-700 hover:border-gray-600 text-gray-400 hover:text-gray-200 rounded-lg text-sm transition-colors"
                    title="海外备用下载"
                  >
                    <ExternalLink className="w-3.5 h-3.5" /> 备用
                  </a>
                </div>
              </div>
            </div>
          );
        })}
      </div>

      <div className="bg-gray-900 border border-gray-800 rounded-xl p-5">
        <h2 className="text-lg font-semibold text-white mb-3">一键安装脚本</h2>
        <div className="space-y-3">
          <div>
            <div className="text-xs text-gray-400 mb-1">Linux / macOS</div>
            <code className="block bg-gray-950 border border-gray-800 rounded-lg px-4 py-2.5 text-sm text-amber-400 font-mono select-all">
              curl -fsSL https://nydus.starclaw.net/spore/install.sh | sh
            </code>
          </div>
          <div>
            <div className="text-xs text-gray-400 mb-1">Windows (PowerShell)</div>
            <code className="block bg-gray-950 border border-gray-800 rounded-lg px-4 py-2.5 text-sm text-amber-400 font-mono select-all">
              irm https://nydus.starclaw.net/spore/install.ps1 | iex
            </code>
          </div>
        </div>
      </div>

      <div className="text-xs text-gray-500 space-y-1">
        <p>• 默认下载地址为国内加速镜像，"备用"为海外节点</p>
        <p>• 安装后在「模型管理」页面配置 API Key 即可开始使用</p>
        <p>• 卸载方法：运行 <code className="text-gray-400 bg-gray-800 px-1.5 py-0.5 rounded">StarClaw-Setup --uninstall</code></p>
      </div>
    </div>
  );
}
