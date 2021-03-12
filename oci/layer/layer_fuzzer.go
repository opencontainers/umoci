// +build gofuzz

/*
 * umoci: Umoci Modifies Open Containers' Images
 * Copyright (C) 2021 SUSE LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *    http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package layer

import (
	"archive/tar"

	"io/ioutil"
	"os"
	"path/filepath"
	"unicode"

	fuzzheaders "github.com/AdamKorcz/go-fuzz-headers"
	"github.com/vbatts/go-mtree"
)

func createRandomFile(dirpath string, filename []byte, filecontents []byte) error {
	fileP := filepath.Join(dirpath, string(filename))
	if err := ioutil.WriteFile(fileP, filecontents, 0644); err != nil {
		return err
	}
	defer os.Remove(fileP)
	return nil
}

func createRandomDir(basedir string, dirname []byte, dirArray []string) ([]string, error) {
	dirPath := filepath.Join(basedir, string(dirname))
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return dirArray, err
	}
	defer os.RemoveAll(dirPath)
	dirArray = append(dirArray, string(dirname))
	return dirArray, nil
}

func isLetter(input []byte) bool {
	s := string(input)
	for _, r := range s {
		if !unicode.IsLetter(r) {
			return false
		}
	}
	return true
}

// FuzzGenerateLayer fuzzes layer.GenerateLayer().
func FuzzGenerateLayer(data []byte) int {
	if len(data) < 5 {
		return -1
	}
	if !fuzzheaders.IsDivisibleBy(len(data), 2) {
		return -1
	}
	half := len(data) / 2
	firstHalf := data[:half]
	f1 := fuzzheaders.NewConsumer(firstHalf)
	err := f1.Split(3, 30)
	if err != nil {
		return -1
	}

	secondHalf := data[half:]
	f2 := fuzzheaders.NewConsumer(secondHalf)
	err = f2.Split(3, 30)
	if err != nil {
		return -1
	}
	baseDir := "fuzz-base-dir"
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return -1
	}
	defer os.RemoveAll(baseDir)

	var dirArray []string
	iteration := 0
	chunkSize := len(f1.RestOfArray) / f1.NumberOfCalls
	for i := 0; i < len(f1.RestOfArray); i = i + chunkSize {
		from := i           //lower
		to := i + chunkSize //upper
		inputData := firstHalf[from:to]
		if len(inputData) > 6 && isLetter(inputData[:5]) {
			dirArray, err = createRandomDir(baseDir, inputData[:5], dirArray)
			if err != nil {
				continue
			}
		} else {
			if len(dirArray) == 0 {
				continue
			}
			dirp := int(inputData[0]) % len(dirArray)
			fp := filepath.Join(baseDir, dirArray[dirp])
			if len(inputData) > 10 {
				filename := inputData[5:8]
				err = createRandomFile(fp, filename, inputData[8:])
				if err != nil {
					continue
				}
			}
		}
		iteration++
	}

	// Get initial.
	initDh, err := mtree.Walk(baseDir, nil, append(mtree.DefaultKeywords, "sha256digest"), nil)
	if err != nil {
		return 0
	}
	iteration = 0
	chunkSize = len(f2.RestOfArray) / f2.NumberOfCalls
	for i := 0; i < len(f2.RestOfArray); i = i + chunkSize {
		from := i           //lower
		to := i + chunkSize //upper
		inputData := secondHalf[from:to]
		if len(inputData) > 6 && isLetter(inputData[:5]) {
			dirArray, err = createRandomDir(baseDir, inputData[:5], dirArray)
			if err != nil {
				continue
			}
		} else {
			if len(dirArray) == 0 {
				continue
			}
			dirp := int(inputData[0]) % len(dirArray)
			fp := filepath.Join(baseDir, dirArray[dirp])
			if len(inputData) > 10 {
				filename := inputData[5:8]
				err = createRandomFile(fp, filename, inputData[8:])
				if err != nil {
					continue
				}
			}
		}
		iteration++
	}

	// Get post.
	postDh, err := mtree.Walk(baseDir, nil, initDh.UsedKeywords(), nil)
	if err != nil {
		return 0
	}

	diffs, err := mtree.Compare(initDh, postDh, initDh.UsedKeywords())
	if err != nil {
		return -1
	}
	reader, err := GenerateLayer(baseDir, diffs, &RepackOptions{})
	if err != nil {
		return -1
	}
	defer reader.Close()

	tr := tar.NewReader(reader)
	for {
		_, err = tr.Next()
		if err != nil {
			break
		}
	}
	return 1
}
