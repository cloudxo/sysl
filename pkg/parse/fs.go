package parse

import (
	"bytes"
	"io"

	"github.com/antlr/antlr4/runtime/Go/antlr"
	"github.com/anz-bank/sysl/pkg/mod"
	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"
)

func fileExists(filename string, fs afero.Fs) bool {
	f, err := fs.Open(filename)
	if err != nil {
		return false
	}
	defer f.Close()
	logrus.Debugf("opened file %s", f.Name())

	_, err = f.Stat()
	return err == nil
}

type fsFileStream struct {
	*antlr.InputStream
	filename string
}

func newFSFileStream(filename string, fs afero.Fs) (s *fsFileStream, m *mod.Module, err error) {
	var f afero.File
	switch t := fs.(type) {
	case *mod.Fs:
		f, m, err = t.OpenWithModule(filename)
	default:
		f, err = fs.Open(filename)
	}

	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	var buf bytes.Buffer
	if _, err = io.Copy(&buf, f); err != nil {
		return nil, nil, err
	}

	return &fsFileStream{antlr.NewInputStream(buf.String()), filename}, m, nil
}
