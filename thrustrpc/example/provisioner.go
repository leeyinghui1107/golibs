package main

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/miketheprogrammer/go-thrust/lib/spawn"
)

type ThrustProvisioner struct{}

func executableNotExist() bool {
	_, err := os.Stat(spawn.GetExecutablePath())
	return os.IsNotExist(err)
}

func (prov ThrustProvisioner) Provision() error {
	basedir := filepath.Join(os.TempDir(), "guivbrdc")
	os.Mkdir(basedir, os.ModeDir)
	spawn.SetBaseDirectory(basedir)
	if executableNotExist() {
		return prov.extractToPath(spawn.GetThrustDirectory())
	}
	return nil
}

func (prov ThrustProvisioner) extractToPath(dest string) error {
	data, err := Asset("asset/thrust_bin.zip")
	if err != nil {
		fmt.Println("Error accessing thrust bindata")
		return err
	}

	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return err
	}
	fmt.Println("Unzipping to", dest)

	for _, f := range r.File {
		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer rc.Close()

		filePath := filepath.Join(dest, f.Name)
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(filePath, 0775); err != nil {
				return err
			}
		} else {
			if err := os.MkdirAll(filepath.Dir(filePath), 0775); err != nil {
				return err
			}
			file, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.FileInfo().Mode())
			if err != nil {
				return err
			}
			defer file.Close()

			_, err = io.Copy(file, rc)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
