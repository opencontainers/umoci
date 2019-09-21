/*
 * umoci: Umoci Modifies Open Containers' Images
 * Copyright (C) 2016-2019 SUSE LLC.
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

package v2

import (
	"github.com/openSUSE/umoci/oci/casext/mediatype"
	"github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/restic/chunker"
)

// OCIv2 media-types.
const (
	MediaTypeRoot  = "application/x-umoci/ociv2.snapshot.root+json"
	MediaTypeChunk = "application/x-umoci/ociv2.snapshot.chunk.v0+raw"
)

// ChunkPolynomial is the only permitted polynomial to be used for the
// Rabbin-fingerprint chunking of FileInode. The canonical implementation of
// this chunking system is available at <https://github.com/restic/chunker>.
const ChunkPolynomial = chunker.Pol(0x2FCEE57A92DE81)

const ChunkMin = 2 * 1024 * 1024
const ChunkMax = 16 * 1024 * 1024

// Timespec ...
type Timespec struct {
	Sec  int64  `json:"s"`
	Nsec uint32 `json:"ns,omitempty"`
}

// InodeMeta ...
type InodeMeta struct {
	UID        uint32   `json:"uid"`
	GID        uint32   `json:"gid"`
	Mode       *uint32  `json:"mode,omitempty"`
	AccessTime Timespec `json:"atime"`
	ModifyTime Timespec `json:"mtime"`
	// TODO: attrs
	// TODO: xattrs
}

// InodeType ...
type InodeType string

// InodeType media-types.
// TODO: Document the meaning of InlineData and IndirectData for each type.
const (
	InodeTypeFile        InodeType = "unix/file.v0"
	InodeTypeDirectory   InodeType = "unix/directory.v0"
	InodeTypeSymlink     InodeType = "unix/symlink.v0"
	InodeTypeCharDevice  InodeType = "unix/char-device.v0"
	InodeTypeBlockDevice InodeType = "unix/block-device.v0"
	InodeTypeNamedPipe   InodeType = "unix/fifo.v0"
	InodeTypeSocket      InodeType = "unix/socket.v0"
)

// Inode is a representation of a generic inode of a given InodeType. The
// meaning of InlineData and IndirectData is incredibly dependent on the
// InodeType.
type Inode struct {
	// What is the type of the
	Type InodeType

	// Meta is used for all inode types, so is inlined.
	Meta InodeMeta `json:"meta,omitempty"`

	// InlineData is for data which can be trivially inlined as a string. The
	// meaning of keys and values is very dependent on Type (and is
	// unfortunately required because of Go not having a typed-union or
	// Rust-like enum concept).
	// XXX: Needs to be made consistent output.
	InlineData map[string]string `json:"inline,omitempty"`

	// IndirectData is for data which is stored as separate blobs. The meaning
	// of this field is very dpeendent on Type.
	IndirectData []v1.Descriptor `json:"indirect,omitempty"`
}

type Root struct {
	// Inodes is the set of all paths and their types in the image.
	// XXX: Needs to be made consistent output.
	Inodes map[string]Inode `json:"content"`
}

func init() {
	mediatype.RegisterParser(MediaTypeRoot, mediatype.CustomJSONParser(Root{}))
}
