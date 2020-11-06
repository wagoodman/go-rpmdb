package rpmdb

import (
	"bytes"
	"encoding/binary"
	"golang.org/x/xerrors"
	"strings"
)

type PackageInfo struct {
	Epoch     int
	Name      string
	Version   string
	Release   string
	Arch      string
	SourceRpm string
	Size      int
	License   string
	Vendor    string
	Files     []FileInfo
}

type FileInfo struct {
	Path   string
	Mode   uint16
	SHA256 string
	Size   int32
}

const (
	// rpmTag_e
	// ref. https://github.com/rpm-software-management/rpm/blob/rpm-4.11.3-release/lib/rpmtag.h#L28
	RPMTAG_NAME        = 1000 /* s */
	RPMTAG_VERSION     = 1001 /* s */
	RPMTAG_RELEASE     = 1002 /* s */
	RPMTAG_EPOCH       = 1003 /* i */
	RPMTAG_ARCH        = 1022 /* s */
	RPMTAG_SOURCERPM   = 1044 /* s */
	RPMTAG_SIZE        = 1009 /* i */
	RPMTAG_LICENSE     = 1014 /* s */
	RPMTAG_VENDOR      = 1011 /* s */
	RPMTAG_DIRINDEXES  = 1116 /* i[] */
	RPMTAG_BASENAMES   = 1117 /* s[] */
	RPMTAG_DIRNAMES    = 1118 /* s[] */
	RPMTAG_FILESIZES   = 1028 /* i[] */
	RPMTAG_FILEMODES   = 1030 /* h[] , specifically []uint16 (ref https://github.com/rpm-software-management/rpm/blob/2153fa4ae51a84547129b8ebb3bb396e1737020e/lib/rpmtypes.h#L53 )*/
	RPMTAG_FILEDIGESTS = 1035 /* s[] */

	//rpmTagType_e
	// ref. https://github.com/rpm-software-management/rpm/blob/rpm-4.11.3-release/lib/rpmtag.h#L362
	RPM_NULL_TYPE         = 0
	RPM_CHAR_TYPE         = 1
	RPM_INT8_TYPE         = 2
	RPM_INT16_TYPE        = 3
	RPM_INT32_TYPE        = 4
	RPM_INT64_TYPE        = 5
	RPM_STRING_TYPE       = 6
	RPM_BIN_TYPE          = 7
	RPM_STRING_ARRAY_TYPE = 8
	RPM_I18NSTRING_TYPE   = 9
)

const (
	sizeOfInt32  = 4
	sizeOfUInt16 = 2
)

func parseStringArray(data []byte) []string {
	elements := strings.Split(string(data), "\x00")
	if len(elements) > 0 && elements[len(elements)-1] == "" {
		return elements[:len(elements)-1]
	}
	return elements
}

func parseString(data []byte) string {
	return string(bytes.TrimRight(data, "\x00"))
}

func parseInt32(data []byte) (int, error) {
	var value int32
	reader := bytes.NewReader(data)
	if err := binary.Read(reader, binary.BigEndian, &value); err != nil {
		return 0, xerrors.Errorf("failed to read binary: %w", err)
	}
	return int(value), nil
}

func parseInt32Array(data []byte, arraySize int) ([]int32, error) {
	var length = arraySize / sizeOfInt32
	values := make([]int32, length)
	reader := bytes.NewReader(data)
	if err := binary.Read(reader, binary.BigEndian, &values); err != nil {
		return nil, xerrors.Errorf("failed to read binary: %w", err)
	}
	return values, nil
}

func parseUInt16Array(data []byte, arraySize int) ([]uint16, error) {
	var length = arraySize / sizeOfUInt16
	values := make([]uint16, length)
	reader := bytes.NewReader(data)
	if err := binary.Read(reader, binary.BigEndian, &values); err != nil {
		return nil, xerrors.Errorf("failed to read binary: %w", err)
	}
	return values, nil
}

// ref. https://github.com/rpm-software-management/rpm/blob/rpm-4.11.3-release/lib/tagexts.c#L649
func newPackage(indexEntries []indexEntry) (*PackageInfo, error) {
	pkgInfo := &PackageInfo{}
	var err error

	for _, indexEntry := range indexEntries {
		switch indexEntry.Info.Tag {
		case RPMTAG_NAME:
			if indexEntry.Info.Type != RPM_STRING_TYPE {
				return nil, xerrors.New("invalid tag name")
			}
			pkgInfo.Name = parseString(indexEntry.Data)
		case RPMTAG_EPOCH:
			if indexEntry.Info.Type != RPM_INT32_TYPE {
				return nil, xerrors.New("invalid tag epoch")
			}

			pkgInfo.Epoch, err = parseInt32(indexEntry.Data)
			if err != nil {
				return nil, xerrors.Errorf("failed to parse epoch: %w", err)
			}
		case RPMTAG_VERSION:
			if indexEntry.Info.Type != RPM_STRING_TYPE {
				return nil, xerrors.New("invalid tag version")
			}
			pkgInfo.Version = parseString(indexEntry.Data)
		case RPMTAG_RELEASE:
			if indexEntry.Info.Type != RPM_STRING_TYPE {
				return nil, xerrors.New("invalid tag release")
			}
			pkgInfo.Release = parseString(indexEntry.Data)
		case RPMTAG_ARCH:
			if indexEntry.Info.Type != RPM_STRING_TYPE {
				return nil, xerrors.New("invalid tag arch")
			}
			pkgInfo.Arch = parseString(indexEntry.Data)
		case RPMTAG_SOURCERPM:
			if indexEntry.Info.Type != RPM_STRING_TYPE {
				return nil, xerrors.New("invalid tag sourcerpm")
			}
			pkgInfo.SourceRpm = parseString(indexEntry.Data)
			if pkgInfo.SourceRpm == "(none)" {
				pkgInfo.SourceRpm = ""
			}
		case RPMTAG_LICENSE:
			if indexEntry.Info.Type != RPM_STRING_TYPE {
				return nil, xerrors.New("invalid tag license")
			}
			pkgInfo.License = parseString(indexEntry.Data)
			if pkgInfo.License == "(none)" {
				pkgInfo.License = ""
			}
		case RPMTAG_VENDOR:
			if indexEntry.Info.Type != RPM_STRING_TYPE {
				return nil, xerrors.New("invalid tag vendor")
			}
			pkgInfo.Vendor = parseString(indexEntry.Data)
			if pkgInfo.Vendor == "(none)" {
				pkgInfo.Vendor = ""
			}
		case RPMTAG_SIZE:
			if indexEntry.Info.Type != RPM_INT32_TYPE {
				return nil, xerrors.New("invalid tag size")
			}

			pkgInfo.Size, err = parseInt32(indexEntry.Data)
			if err != nil {
				return nil, xerrors.Errorf("failed to parse size: %w", err)
			}
		}
	}

	files, err := getFileInfo(indexEntries)
	if err != nil {
		return nil, xerrors.Errorf("failed to read package files: %w", err)
	}

	pkgInfo.Files = files

	return pkgInfo, nil
}

func getFileInfo(indexEntries []indexEntry) ([]FileInfo, error) {
	var err error

	// each of these fields are arrays of metadata for a single file, where the same index across variables are
	// for the same file (this is how the information is stored within the RPM DB)
	var allBasenames []string
	var allDirs []string
	var allDirIndexes []int32
	var allFileDigests []string
	var allFileModes []uint16
	var allFileSizes []int32

	for _, indexEntry := range indexEntries {
		switch indexEntry.Info.Tag {

		case RPMTAG_FILESIZES:
			// note: there is no distinction between int32, uint32, and []uint32
			if indexEntry.Info.Type != RPM_INT32_TYPE {
				return nil, xerrors.New("invalid tag file-sizes")
			}
			allFileSizes, err = parseInt32Array(indexEntry.Data, indexEntry.Length)
			if err != nil {
				return nil, xerrors.Errorf("failed to parse file-sizes: %w", err)
			}
		case RPMTAG_FILEDIGESTS:
			if indexEntry.Info.Type != RPM_STRING_ARRAY_TYPE {
				return nil, xerrors.New("invalid tag file-digests")
			}
			allFileDigests = parseStringArray(indexEntry.Data)
		case RPMTAG_FILEMODES:
			// note: there is no distinction between int16, uint16, and []uint16
			if indexEntry.Info.Type != RPM_INT16_TYPE {
				return nil, xerrors.New("invalid tag file-modes")
			}
			allFileModes, err = parseUInt16Array(indexEntry.Data, indexEntry.Length)
			if err != nil {
				return nil, xerrors.Errorf("failed to parse file-modes: %w", err)
			}
		case RPMTAG_BASENAMES:
			if indexEntry.Info.Type != RPM_STRING_ARRAY_TYPE {
				return nil, xerrors.New("invalid tag basenames")
			}
			allBasenames = parseStringArray(indexEntry.Data)
		case RPMTAG_DIRNAMES:
			if indexEntry.Info.Type != RPM_STRING_ARRAY_TYPE {
				return nil, xerrors.New("invalid tag dir-names")
			}
			allDirs = parseStringArray(indexEntry.Data)
		case RPMTAG_DIRINDEXES:
			// note: there is no distinction between int32, uint32, and []uint32
			if indexEntry.Info.Type != RPM_INT32_TYPE {
				return nil, xerrors.New("invalid tag dir-indexes")
			}
			allDirIndexes, err = parseInt32Array(indexEntry.Data, indexEntry.Length)
			if err != nil {
				return nil, xerrors.Errorf("failed to parse dir-indexes: %w", err)
			}
		}
	}

	// now that we have all of the available metadata, piece together a list of files and their metadata
	var files []FileInfo
	if allDirs != nil && allDirIndexes != nil {
		for i, file := range allBasenames {
			var digest string
			var mode uint16
			var size int32

			if allFileDigests != nil && len(allFileDigests) > i {
				digest = allFileDigests[i]
			}

			if allFileModes != nil && len(allFileModes) > i {
				mode = allFileModes[i]
			}

			if allFileSizes != nil && len(allFileSizes) > i {
				size = allFileSizes[i]
			}

			record := FileInfo{
				Path:   allDirs[allDirIndexes[i]] + file,
				Mode:   mode,
				SHA256: digest,
				Size:   size,
			}
			files = append(files, record)
		}
	}

	return files, nil
}
