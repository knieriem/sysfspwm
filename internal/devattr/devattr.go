package devattr

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
)

const debug = false

type File struct {
	f         io.ReadWriteCloser
	value     int64
	debugInfo func(data string)
}

func Open(dir, attrName string, flags int) (*File, error) {
	f, err := os.OpenFile(filepath.Join(dir, attrName), flags, 0)
	if err != nil {
		return nil, err
	}
	attr := &File{f: f, value: -1}
	if debug {
		basename := filepath.Base(dir)
		attr.debugInfo = func(data string) {
			fmt.Printf("%s/%s: %q\n", basename, attrName, data)
		}
	}
	return attr, nil
}

func (attr *File) Close() error {
	return attr.f.Close()
}

func (attr *File) Int64() int64 {
	return attr.value
}

func (attr *File) IsZero() bool {
	return attr.value == 0
}

func (attr *File) WriteInt64(i int64) error {
	if attr.value == i {
		return nil
	}
	attr.value = i
	_, err := attr.write([]byte(strconv.FormatInt(i, 10)))
	return err
}

func (attr *File) Write0() error {
	if attr.value == 0 {
		return nil
	}
	attr.value = 0
	return attr.writeByte('0')
}

func (attr *File) Write1() error {
	if attr.value == 1 {
		return nil
	}
	attr.value = 1
	return attr.writeByte('1')
}

func (attr *File) writeByte(c byte) error {
	_, err := attr.write([]byte{c})
	return err
}

func (attr *File) write(b []byte) (int, error) {
	if debug {
		attr.debugInfo(string(b))
	}
	return attr.f.Write(b)
}

func (attr *File) ReadInt() (int, error) {
	var i int64
	_, err := fmt.Fscan(attr.f, &i)
	if err != nil {
		return 0, err
	}
	attr.value = i
	return int(i), nil
}

func ReadIntFile(dir, attrName string) (int, error) {
	f, err := Open(dir, attrName, os.O_RDONLY)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	return f.ReadInt()
}

func WriteIntFile(dir, attrName string, value int) error {
	f, err := Open(dir, attrName, os.O_WRONLY)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.WriteInt64(int64(value))
}
