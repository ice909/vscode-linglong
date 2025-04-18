package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/aptly-dev/aptly/aptly"
	"github.com/aptly-dev/aptly/console"
	"github.com/aptly-dev/aptly/deb"
	"github.com/aptly-dev/aptly/files"
	"github.com/aptly-dev/aptly/http"
	"github.com/aptly-dev/aptly/query"
	"github.com/aptly-dev/aptly/utils"
)

func main() {
	err := gen(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}
}

var GenDebSource = "# linglong:gen_deb_source "

func gen(filename string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}
	lines := bytes.Split(data, []byte{'\n'})
	var endLine int
	var arch, repoUrl, codename string
	var components, filter []string

	for index := range lines {
		line := strings.TrimSpace(string(lines[index]))
		if !strings.HasPrefix(line, GenDebSource) {
			continue
		}
		endLine = index
		line = line[len(GenDebSource):]
		if strings.HasPrefix(line, "install") {
			filter = append(filter, strings.ReplaceAll(line[len("install "):], ",", "|"))
		} else if strings.HasPrefix(line, "sources") {
			fields := strings.Fields(line)
			if len(fields) < 3 {
				continue
			}
			arch = fields[1]
			repoUrl = fields[2]
			codename = fields[3]
			components = fields[4:]
		}
	}

	pkg, err := getPkg(arch, repoUrl, codename, components, strings.Join(filter, "|"))
	if err != nil {
		return err
	}
	lines = lines[:endLine+1]
	sort.SliceStable(pkg, func(i, j int) bool {
		c := strings.Compare(pkg[i].File.Filename, pkg[j].File.Filename)
		return c < 0
	})
	for i := range pkg {
		source := fmt.Sprintf("  - kind: file\n    url: %s/%s\n    digest: %s",
			repoUrl, pkg[i].File.DownloadURL(), pkg[i].File.Checksums.SHA256,
		)
		lines = append(lines, []byte(source))
	}
	data = bytes.Join(lines, []byte{'\n'})
	err = os.WriteFile(filename, data, 0600)
	if err != nil {
		return fmt.Errorf("save file: %w", err)
	}
	return nil
}

type DownloadWithCache struct {
	aptly.Downloader
}

func (cache *DownloadWithCache) DownloadWithChecksum(ctx context.Context, url string, destination string,
	expected *utils.ChecksumInfo, ignoreMismatch bool) error {
	d := sha256.Sum256([]byte(url))
	cacheFile := filepath.Join(os.TempDir(), "aptly_dl_cache_"+hex.EncodeToString(d[:]))
	stat, err := os.Stat(cacheFile)
	if err != nil {
		// 缓存文件不存在则下载，其他错误直接返回
		if !os.IsNotExist(err) {
			return fmt.Errorf("open cache file: %w", err)
		}
	}
	if stat != nil {
		// 比较缓存文件和远程文件是否大小一样，如果一样就认为缓存没有过期
		remoteSize, err := cache.Downloader.GetLength(ctx, url)
		if err != nil {
			return fmt.Errorf("get file size: %w", err)
		}
		if stat.Size() == remoteSize {
			log.Println("Downloading", url)
			log.Println("use cache", cacheFile)
			return utils.CopyFile(cacheFile, destination)
		}
	}
	err = cache.Downloader.DownloadWithChecksum(ctx, url, destination, expected, ignoreMismatch)
	if err != nil {
		return err
	}
	// 复制下载的文件到缓存
	return utils.CopyFile(destination, cacheFile)
}

func getPkg(arch, repoURL, distribution string, components []string, filter string) ([]deb.PackageDownloadTask, error) {
	repo, err := deb.NewRemoteRepo(
		"", repoURL, distribution, components, []string{arch},
		false, false, false)
	if err != nil {
		return nil, fmt.Errorf("new remote repo: %w", err)
	}
	progress := console.NewProgress()
	progress.Start()
	repo.FilterWithDeps = true
	filterQuery, err := query.Parse(filter)
	if err != nil {
		return nil, fmt.Errorf("parse filter: %s", err)
	}
	downloader := http.NewDownloader(0, 1, progress)
	err = repo.DownloadPackageIndexes(progress, &DownloadWithCache{Downloader: downloader}, nil, nil, true)
	if err != nil {
		return nil, fmt.Errorf("download package index: %s", err)
	}
	oldLen, newLen, err := repo.ApplyFilter(0, filterQuery, progress)
	if err != nil {
		return nil, fmt.Errorf("apply filter: %s", err)
	}
	log.Println("total:", oldLen, "selected:", newLen)

	packagePool := files.NewPackagePool("./", false)
	queue, _, err := repo.BuildDownloadQueue(packagePool, nil,
		nil, true)
	if err != nil {
		return nil, fmt.Errorf("build download queue: %w", err)
	}
	progress.Flush()
	return queue, nil
}
