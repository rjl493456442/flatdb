package flatdb

import (
	"fmt"
	"os"
)

func ExampleFlatDB() {
	// Initialize the empty flatdb with "write-only" mode.
	flatdb, err := NewFlatDatabase("path/to/flatdb", false)
	if err != nil {
		fmt.Printf("Failed to initialize flatdb, error: %v\n", err)
	}
	defer os.RemoveAll("path/to/flatdb")

	flatdb.Put([]byte{0x01, 0x02, 0x03}, []byte{0x1a, 0x1b, 0x1c})
	flatdb.Put([]byte{0x04, 0x05, 0x06}, []byte{0x2a, 0x2b, 0x2c})
	flatdb.Put([]byte{0x07, 0x08, 0x09}, []byte{0x3a, 0x3b, 0x3c})

	// Call the commit to flush out all the database content.
	// In the mean time the database is converted into the
	// "read-only" mode.
	if err := flatdb.Commit(); err != nil {
		fmt.Printf("Failed to commit flatdb, error: %v\n", err)
	}
	//  Random read is not allowed!!
	//
	//	flatdb.Get([]byte{0x01, 0x02, 0x03})

	// The only meaningful way to "read" the database is using
	// the iteration. The iteration order is exactly same with
	// the write order.
	iter := flatdb.NewIterator(nil, nil)
	defer iter.Release()

	for iter.Next() {
		fmt.Println(iter.Key())
		fmt.Println(iter.Value())
	}
	// Output:
	// [1 2 3]
	// [26 27 28]
	// [4 5 6]
	// [42 43 44]
	// [7 8 9]
	// [58 59 60]
}
