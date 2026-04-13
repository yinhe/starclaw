package knowledge

import "github.com/yinhe/starclaw/internal/sandbox"

func getUploadDir() string {
	return sandbox.UploadsDir()
}
