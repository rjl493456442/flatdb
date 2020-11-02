// Copyright (c) 2020, Gary Rong <garyrong0905@gmail.com>
// All rights reserved.
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package flatdb

import (
	"bytes"
	"crypto/rand"
	"io/ioutil"
	"os"
	"sync"
	"testing"
)

type flatDBTester struct {
	dir string
	db  *FlatDatabase
}

func newFlatDBTester(read bool) *flatDBTester {
	dir, _ := ioutil.TempDir("", "")
	db, err := NewFlatDatabase(dir, read)
	if err != nil {
		return nil
	}
	return &flatDBTester{
		dir: dir,
		db:  db,
	}
}

func (tester *flatDBTester) teardown() {
	if tester.dir != "" {
		os.RemoveAll(tester.dir)
	}
}

func (tester *flatDBTester) Put(key, value []byte) {
	tester.db.Put(key, value)
}

func (tester *flatDBTester) Iterate() *FlatIterator {
	return tester.db.NewIterator(nil, nil)
}

func (tester *flatDBTester) Commit() {
	tester.db.Commit()
}

func (tester *flatDBTester) checkIteration(t *testing.T, keys [][]byte, vals [][]byte) {
	iter := tester.Iterate()
	var index int
	for iter.Next() {
		if index >= len(keys) {
			t.Fatalf("Extra entry found")
		}
		if index >= len(vals) {
			t.Fatalf("Extra entry found")
		}
		if !bytes.Equal(iter.Key(), keys[index]) {
			t.Fatalf("Entry key mismatch %v -> %v", keys[index], iter.Key())
		}
		if !bytes.Equal(iter.Value(), vals[index]) {
			t.Fatalf("Entry value mismatch %v -> %v", vals[index], iter.Value())
		}
		index += 1
	}
	if iter.Error() != nil {
		t.Fatalf("Iteration error %v", iter.Error())
	}
	iter.Release()

	if index != len(keys) {
		t.Fatalf("Missing entries, want %d, got %d", len(keys), index)
	}
}

func (tester *flatDBTester) checkIterationNoOrder(t *testing.T, entries map[string][]byte) {
	iter := tester.Iterate()
	var index int
	for iter.Next() {
		val, ok := entries[string(iter.Key())]
		if !ok {
			t.Fatalf("Unexpected key %v", iter.Key())
		}
		if !bytes.Equal(iter.Value(), val) {
			t.Fatalf("Entry value mismatch %v -> %v", val, iter.Value())
		}
		index += 1
	}
	if iter.Error() != nil {
		t.Fatalf("Iteration error %v", iter.Error())
	}
	iter.Release()

	// The assumption is held there is no duplicated entry.
	if index != len(entries) {
		t.Fatalf("Missing entries, want %d, got %d", len(entries), index)
	}
}

func newTestCases(size int) ([][]byte, [][]byte) {
	var (
		keys [][]byte
		vals [][]byte
		kbuf [20]byte
		vbuf [32]byte
	)
	for i := 0; i < size; i++ {
		rand.Read(kbuf[:])
		keys = append(keys, CopyBytes(kbuf[:]))

		rand.Read(vbuf[:])
		vals = append(vals, CopyBytes(vbuf[:]))
	}
	return keys, vals
}

func TestReadNonExistentDB(t *testing.T) {
	tester := newFlatDBTester(true)
	if tester != nil {
		t.Fatalf("Expect the error for opening the non-existent db")
	}
}

func TestReadConcurrently(t *testing.T) {
	tester := newFlatDBTester(false)
	if tester == nil {
		t.Fatalf("Failed to init tester")
	}
	defer tester.teardown()

	iter := tester.Iterate()
	if iter == nil {
		t.Fatalf("Failed to obtain iterator")
	}
	if iter := tester.Iterate(); iter != nil {
		t.Fatalf("Concurrent iteration is not allowed")
	}
	iter.Release()
	if iter := tester.Iterate(); iter == nil {
		t.Fatalf("Failed to obtain iterator")
	}
}

func TestFlatDatabase(t *testing.T) {
	tester := newFlatDBTester(false)
	if tester == nil {
		t.Fatalf("Failed to init tester")
	}
	defer tester.teardown()

	keys, vals := newTestCases(1024 * 1024)
	for i := 0; i < len(keys); i++ {
		tester.Put(keys[i], vals[i])
	}
	tester.Commit()
	tester.checkIteration(t, keys, vals)
	tester.checkIteration(t, keys, vals) // Check twice
}

func TestFlatDatabaseBatchWrite(t *testing.T) {
	tester := newFlatDBTester(false)
	if tester == nil {
		t.Fatalf("Failed to init tester")
	}
	defer tester.teardown()

	keys, vals := newTestCases(1024 * 1024)
	batch := tester.db.NewBatch()
	for i := 0; i < len(keys); i++ {
		batch.Put(keys[i], vals[i])
		if batch.ValueSize() > 1024 {
			batch.Write()
			batch.Reset()
		}
	}
	batch.Write()

	tester.Commit()
	tester.checkIteration(t, keys, vals)
}

func TestFlatDatabaseConcurrentWrite(t *testing.T) {
	tester := newFlatDBTester(false)
	if tester == nil {
		t.Fatalf("Failed to init tester")
	}
	defer tester.teardown()

	var (
		wg      sync.WaitGroup
		mixLock sync.Mutex
		mix     = make(map[string][]byte)
	)
	writer := func() {
		defer wg.Done()
		keys, vals := newTestCases(1024 * 1024)
		batch := tester.db.NewBatch()
		for i := 0; i < len(keys); i++ {
			batch.Put(keys[i], vals[i])
			if batch.ValueSize() > 1024 {
				batch.Write()
				batch.Reset()
			}
		}
		batch.Write()

		mixLock.Lock()
		for i := 0; i < len(keys); i++ {
			mix[string(keys[i])] = vals[i]
		}
		mixLock.Unlock()
	}
	wg.Add(2)
	for i := 0; i < 2; i++ {
		go writer()
	}
	wg.Wait()
	tester.Commit()
	tester.checkIterationNoOrder(t, mix)
}

// CopyBytes returns an exact copy of the provided bytes.
func CopyBytes(b []byte) (copiedBytes []byte) {
	if b == nil {
		return nil
	}
	copiedBytes = make([]byte, len(b))
	copy(copiedBytes, b)

	return
}
