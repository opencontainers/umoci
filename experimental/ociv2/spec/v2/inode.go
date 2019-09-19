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
	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/restic/chunker"
)

// OCIv2 media-types.
const (
	MediaTypeInodeFile      = "application/x-umoci/ociv2.snapshot.inode/unix-file.v0+json"
	MediaTypeInodeDirectory = "application/x-umoci/ociv2.snapshot.inode/unix-directory.v0+json"
	MediaTypeInodeSymlink   = "application/x-umoci/ociv2.snapshot.inode/unix-symlink.v0+json"
	MediaTypeInodeDevice    = "application/x-umoci/ociv2.snapshot.inode/unix-device.v0+json"
	MediaTypeInodeNamedPipe = "application/x-umoci/ociv2.snapshot.inode/unix-fifo.v0+json"
	MediaTypeInodeSocket    = "application/x-umoci/ociv2.snapshot.inode/unix-socket.v0+json"
	//MediaTypeInodeHardlink  = "application/x-umoci/ociv2.snapshot.inode/unix-hardlink.v0+json"

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
	Nsec uint32 `json:"ns"`
}

// InodeMeta ...
type InodeMeta struct {
	UID        uint32   `json:"uid"`
	GID        uint32   `json:"gid"`
	Mode       *uint32  `json:"mode,omitempty"`
	AccessTime Timespec `json:"atime"`
	ModifyTime Timespec `json:"mtime"`
	ChangeTime Timespec `json:"ctime"`
	// TODO: attrs
	// TODO: xattrs
}

// BasicInode is the core inode structure that is embedded in all other inode
// types. All inodes have this metadata.
type BasicInode struct {
	// Meta is a the
	Meta InodeMeta `json:"meta,omitempty"`
}

// FileInode represents the inode of an ordinary file. The contents of the file
// are represented by the Chunks list (which are a set of CDC-chunked .
type FileInode struct {
	BasicInode
	// Digest is the complete digest of all of the chunk data. This allows for
	// verification of the final file's contents, and for file-store
	// deduplication to be conducted even if two image generators have
	// different chunking algorithms (even though this shouldn't happen).
	Digest digest.Digest
	// Chunks is the list of (in-order) chunks that make up the file.
	Chunks []v1.Descriptor `json:"chunks,omitempty"`
}

// DirectoryInode represents a directory, with Children being a list of
// Descriptors to inodes inside the given directory.
type DirectoryInode struct {
	BasicInode
	// Children is the map of child entries to inode descriptors.
	// TODO TODO TODO TODO TODO TODO TODO TODO TODO TODO TODO TODO TODO
	// OUTPUT CONSISTENT MAPS.
	// TODO TODO TODO TODO TODO TODO TODO TODO TODO TODO TODO TODO TODO
	Children map[string]v1.Descriptor `json:"children"`
}

// SymlinkInode represents a symlink with the given string content as the
// target. The Mode of this inode *must* be set to nil in order to be a valid
// SymlinkInode.
type SymlinkInode struct {
	BasicInode
	// Target is the symlink target string.
	Target string `json:"target"`
}

// DeviceType is used to represent the type of DeviceInode.
type DeviceType string

const (
	// CharDevice represents a character device DeviceInode.
	CharDevice DeviceType = "char"
	// BlockDevice represents a block device DeviceInode.
	BlockDevice DeviceType = "block"
)

// DeviceInode represents a character or block device inode (with the given
// major and minor numbers).
type DeviceInode struct {
	BasicInode
	// Type is what kind of device this is.
	Type DeviceType `json:"type"`
	// Major is the major number of the device.
	Major uint32 `json:"major"`
	// Major is the minor number of the device.
	Minor uint32 `json:"minor"`
}

// NamedPipeInode repesents a named pipe (or FIFO).
type NamedPipeInode struct {
	BasicInode
}

// SocketInode represents a unix socket.
type SocketInode struct {
	BasicInode
}

func init() {
	mediatype.RegisterParser(MediaTypeInodeFile, mediatype.CustomJSONParser(FileInode{}))
	mediatype.RegisterParser(MediaTypeInodeDirectory, mediatype.CustomJSONParser(DirectoryInode{}))
	mediatype.RegisterParser(MediaTypeInodeSymlink, mediatype.CustomJSONParser(SymlinkInode{}))
	mediatype.RegisterParser(MediaTypeInodeDevice, mediatype.CustomJSONParser(DeviceInode{}))
	mediatype.RegisterParser(MediaTypeInodeNamedPipe, mediatype.CustomJSONParser(NamedPipeInode{}))
	mediatype.RegisterParser(MediaTypeInodeSocket, mediatype.CustomJSONParser(SocketInode{}))
}
