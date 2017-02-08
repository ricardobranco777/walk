// Copyright 2010 Andrey Mirtchovski. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the Go distribution's
// LICENSE file.

// A non-recursive filesystem walker
package walk

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"syscall"
)

type queue struct {
	dir  string
	name string
	seen bool
	cd   bool
	next *queue
}

var lstat = os.Lstat // for testing

// Walkiter iteratively descends through a directory storing subdirectories
// on s and calling walkFn for each file or directory it encounters
func walkiter(s *queue, walkFn filepath.WalkFunc) (haderror error) {
	for {
		if s == nil {
			return haderror
		}
		if s.seen {
			if s.cd {
				err := os.Chdir("..")
				if err != nil {
					cwd, err2 := os.Getwd()
					return fmt.Errorf("can't dotdot (..); : %s from %s: %v (getwd error: %v)",
						filepath.Join(s.dir, s.name), cwd, err, err2)
				}
			}
			s = s.next
			continue
		}
		s.seen = true

		ourname := filepath.Join(s.dir, s.name)

		info, err := lstat(s.name)
		if err != nil {
			haderror = walkFn(ourname, info, err)
			continue
		}

		err = walkFn(ourname, info, err)
		if err != nil {
			if info.IsDir() && err == filepath.SkipDir {
				continue
			}
			haderror = err
			continue
		}

		// if a directory, chdir and list it
		if info.IsDir() {
			err := os.Chdir(s.name)
			if err != nil {
				haderror = walkFn(ourname, info, err)
				continue
			}
			s.cd = true
			file, err := os.Open(".")
			if err != nil {
				haderror = walkFn(ourname, info, err)
				continue
			}
			names, err := file.Readdirnames(0)
			if err != nil {
				haderror = walkFn(ourname, info, err)
			}
			file.Close()
			for _, name := range names {
				ns := new(queue)
				ns.dir = ourname
				ns.name = name
				ns.next = s
				s = ns
			}
		}
	}
	panic("unreachable")
}

// Walk does a non-recursive walk of the directory rooted at path, calling f for
// each file it encounters. The walk will descend using Chdir, so that deeply nested
// paths longer than PATH_MAX (1024 on OSX) would still be reachable.
func Walk(root string, walkFn filepath.WalkFunc) error {
	root = path.Clean(root)
	dir, err := os.Open(".")
	if err != nil {
		return err // wouldn't want to leave caller in an unknown dir
	}
	syscall.Syscall(syscall.SYS_FCNTL, dir.Fd(), syscall.F_SETFD, syscall.FD_CLOEXEC)
	defer syscall.Fchdir(int(dir.Fd()))
	defer dir.Close()

	err = os.Chdir(filepath.Dir(root))
	if err != nil {
		return walkFn(root, nil, err)
	}
	s := new(queue)
	s.name = root
	return walkiter(s, walkFn)
}
