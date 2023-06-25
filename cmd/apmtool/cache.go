// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gofrs/flock"
)

var (
	cacheDir = initCacheDir()
)

// readCache reads a file in the cache directory with a file lock,
// return an error satisfying errors.Is(err, os.ErrNotExist) if the
// cache file does not yet exist.
//
// The filename may not start with a '.'.
func readCache(filename string) ([]byte, error) {
	if strings.HasPrefix(filename, ".") {
		return nil, fmt.Errorf("invalid filename %q, may not start with '.'", filename)
	}

	cacheFlock := newCacheFlock()
	if err := cacheFlock.Lock(); err != nil {
		return nil, fmt.Errorf("error acquiring lock on cache directory: %w", err)
	}
	defer cacheFlock.Unlock()

	cacheFilePath := filepath.Join(cacheDir, filename)
	data, err := os.ReadFile(cacheFilePath)
	if err != nil {
		return nil, fmt.Errorf("error reading cache file: %w", err)
	}
	return data, nil
}

// updateCache reads a file in the cache directory with a file lock,
// passes it to the update function, and then writes the result back
// to the file. If the file does not exist initially, a nil slice is
// passed to the update function.
//
// The filename may not start with a '.'.
func updateCache(filename string, update func([]byte) ([]byte, error)) error {
	if strings.HasPrefix(filename, ".") {
		return fmt.Errorf("invalid filename %q, may not start with '.'", filename)
	}

	cacheFlock := newCacheFlock()
	if err := cacheFlock.Lock(); err != nil {
		return fmt.Errorf("error acquiring lock on cache directory: %w", err)
	}
	defer cacheFlock.Unlock()

	cacheFilePath := filepath.Join(cacheDir, filename)
	cacheFilePathTemp := filepath.Join(cacheDir, ".temp."+filename)
	data, err := os.ReadFile(cacheFilePath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("error reading cache file: %w", err)
		}
		data = nil
	}
	data, err = update(data)
	if err != nil {
		return fmt.Errorf("error updating cache file %q: %w", filename, err)
	}

	// Write temporary file and rename, to prevent partial writes.
	if err := os.WriteFile(cacheFilePathTemp, data, 0600); err != nil {
		return fmt.Errorf("error writing temporary cache file %q: %w", filename, err)
	}
	if err := os.Rename(cacheFilePathTemp, cacheFilePath); err != nil {
		return fmt.Errorf("error renaming temporary cache file %q: %w", filename, err)
	}
	return nil
}

func newCacheFlock() *flock.Flock {
	return flock.New(filepath.Join(cacheDir, ".flock"))
}

type CredentialsCache struct {
}

func initCacheDir() string {
	userCacheDir, err := os.UserCacheDir()
	if err != nil {
		panic(fmt.Errorf("error getting user cache dir: %w", err))
	}
	dir := filepath.Join(userCacheDir, "apmtool")
	if err := os.MkdirAll(dir, 0700); err != nil {
		panic(fmt.Errorf("error creating cache dir: %w", err))
	}
	return dir
}
