package util

import (
	"io"
	"os"
	"os/exec"

	"github.com/pkg/errors"
)

func ExecScriptAndGetFile(script, filePath string) (io.ReadCloser, error) {
	if err := exec.Command("bash", "-c", script).Run(); err != nil {
		return nil, errors.Wrap(err, "exec script failed")
	}
	file, err := os.Open(filePath)
	if err != nil {
		return nil, errors.Wrap(err, "open file failed")
	}
	return file, nil
}
