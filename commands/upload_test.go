package commands

import (
	"os"
	"testing"
)

func TestUpload(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	os.Args = []string{"cmd", "-u", "yinghau76@gmail.com", "upload", "../test/test.md"}
	if err := Execute(); err != nil {
		t.Fatal(err)
	}
}