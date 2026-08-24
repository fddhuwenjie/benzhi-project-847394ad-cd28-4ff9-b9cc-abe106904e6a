package jsonstore

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"confinedpermit/internal/domain"
)

func loadDocument(path string) (*document, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return emptyDocument(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("打开数据快照: %w", err)
	}
	defer f.Close()
	dec := json.NewDecoder(io.LimitReader(f, 64<<20))
	dec.DisallowUnknownFields()
	var d document
	if err := dec.Decode(&d); err != nil {
		return nil, fmt.Errorf("解析数据快照: %w", err)
	}
	if d.Version != documentVersion {
		return nil, fmt.Errorf("不支持的数据快照版本 %d", d.Version)
	}
	if d.Permits == nil {
		d.Permits = map[string]*domain.PermitBundle{}
	}
	if d.Idempotency == nil {
		d.Idempotency = map[string]idempotencyRecord{}
	}
	return &d, nil
}

func saveDocument(path string, d *document) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("创建数据目录: %w", err)
	}
	f, err := os.CreateTemp(dir, ".permit-snapshot-*")
	if err != nil {
		return fmt.Errorf("创建临时快照: %w", err)
	}
	tmp := f.Name()
	ok := false
	defer func() {
		f.Close()
		if !ok {
			os.Remove(tmp)
		}
	}()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(d); err != nil {
		return fmt.Errorf("编码数据快照: %w", err)
	}
	if err := f.Sync(); err != nil {
		return fmt.Errorf("同步临时快照: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("关闭临时快照: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("替换数据快照: %w", err)
	}
	df, err := os.Open(dir)
	if err == nil {
		err = df.Sync()
		df.Close()
	}
	if err != nil {
		return fmt.Errorf("同步数据目录: %w", err)
	}
	ok = true
	return nil
}
