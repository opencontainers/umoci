#!/usr/bin/env bats -t
# SPDX-License-Identifier: Apache-2.0
# umoci: Umoci Modifies Open Containers' Images
# Copyright (C) 2016-2025 SUSE LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#   http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

load helpers

function setup() {
	setup_tmpdirs
	setup_image
}

function teardown() {
	teardown_tmpdirs
	teardown_image
}

@test "umoci repack" {
	# Unpack the image.
	new_bundle_rootfs
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	# Make sure the files we're creating don't exist.
	! [ -e "$ROOTFS/newfile" ]
	! [ -e "$ROOTFS/newdir" ]
	! [ -e "$ROOTFS/newdir/anotherfile" ]
	! [ -e "$ROOTFS/newdir/link" ]

	# Create them.
	echo "first file" > "$ROOTFS/newfile"
	mkdir "$ROOTFS/newdir"
	echo "subfile" > "$ROOTFS/newdir/anotherfile"
	ln -s "this is a dummy symlink" "$ROOTFS/newdir/link"

	# Repack the image under a new tag.
	umoci repack --image "${IMAGE}:${TAG}-new" "$BUNDLE"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Unpack it again.
	new_bundle_rootfs
	umoci unpack --image "${IMAGE}:${TAG}-new" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	# Ensure that gomtree succeeds on the old bundle, which is what this was
	# generated from.
	gomtree -p "$ROOTFS" -f "$BUNDLE"/sha256_*.mtree
	[ "$status" -eq 0 ]
	[ -z "$output" ]

	# Just for sanity, check that everything looks okay.
	[ -f "$ROOTFS/newfile" ]
	[ -d "$ROOTFS/newdir" ]
	[ -f "$ROOTFS/newdir/anotherfile" ]
	[ -L "$ROOTFS/newdir/link" ]

	# Make sure that repack fails without a bundle path.
	umoci repack --image "${IMAGE}:${TAG}-new2"
	[ "$status" -ne 0 ]
	umoci stat --image "${IMAGE}:${TAG}-new2" --json
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"
	# ... or with too many
	umoci repack --image "${IMAGE}:${TAG}-new3" too many arguments
	[ "$status" -ne 0 ]
	umoci stat --image "${IMAGE}:${TAG}-new3" --json
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Make sure we added a new layer.
	umoci stat --image "${IMAGE}:${TAG}" --json
	[ "$status" -eq 0 ]
	numLinesA="$(echo "$output" | jq -SM '.history | length')"

	umoci stat --image "${IMAGE}:${TAG}-new" --json
	[ "$status" -eq 0 ]
	numLinesB="$(echo "$output" | jq -SM '.history | length')"

	# Number of lines should be greater.
	[ "$numLinesB" -gt "$numLinesA" ]
	# Make sure that the new layer is a non-empty_layer.
	[[ "$(echo "$output" | jq -SM '.history[-1].empty_layer')" == "null" ]]

	image-verify "${IMAGE}"
}

@test "umoci repack [invalid arguments]" {
	# Unpack the image.
	new_bundle_rootfs
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	# Missing --image and bundle arguments.
	umoci repack
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Missing --image argument.
	umoci repack "$BUNDLE"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Missing bundle argument.
	umoci repack --image "${IMAGE}:${TAG}"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Empty image path.
	umoci repack --image ":${TAG}" "$BUNDLE"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Non-existent image path.
	umoci repack --image "${IMAGE}-doesnotexist:${TAG}" "$BUNDLE"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Empty image destination tag.
	umoci repack --image "${IMAGE}:" "$BUNDLE"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Invalid image destination tag.
	umoci repack --image "${IMAGE}:${INVALID_TAG}" "$BUNDLE"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Non-existent bundle directory.
	umoci repack --image "${IMAGE}:${TAG}" "$BUNDLE-doesnotexist"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Non-existent rootfs directory.
	mv "$BUNDLE/rootfs" "$BUNDLE/old_rootfs"
	umoci repack --image "${IMAGE}:${TAG}" "$BUNDLE"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"
	mv "$BUNDLE/old_rootfs" "$BUNDLE/rootfs"

	# Unknown flag argument.
	umoci repack --this-is-an-invalid-argument \
		--image="${IMAGE}:${TAG}" "$BUNDLE"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Too many positional arguments.
	umoci repack --image "${IMAGE}:${TAG}" "$BUNDLE" \
		this-is-an-invalid-argument
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# --no-history conflicts with --history.* flags.
	umoci repack --image "${IMAGE}:${TAG}" \
		--no-history --history.author "Violet Beauregarde" "$BUNDLE"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# --history.created has to be an ISO-8601 date.
	umoci repack --image "${IMAGE}:${TAG}" \
		--history.created "invalid date" "$BUNDLE"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# --compress=... has to be a valid value.
	umoci repack --image "${IMAGE}:${TAG}" --compress=invalid "$BUNDLE"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"
}

@test "umoci repack [whiteout]" {
	# Unpack the image.
	new_bundle_rootfs && ROOTFS_A="$ROOTFS"
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	# Make sure the files we're deleting exist.
	[ -d "$ROOTFS/etc" ]
	[ -L "$ROOTFS/bin/sh" ]
	[ -e "$ROOTFS/usr/bin/env" ]

	# Remove them.
	rm_rf "$ROOTFS/"{etc,bin/sh,usr/bin/env}

	# Repack the image under a new tag.
	umoci repack --image "${IMAGE}:${TAG}-new" "$BUNDLE"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Unpack it again.
	new_bundle_rootfs && BUNDLE_B="$BUNDLE"
	umoci unpack --image "${IMAGE}:${TAG}-new" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	# Ensure that gomtree succeeds on the old bundle, which is what this was
	# generated from.
	gomtree -p "$ROOTFS_A" -f "$BUNDLE_B"/sha256_*.mtree
	[ "$status" -eq 0 ]
	[ -z "$output" ]

	# Just for sanity, check that everything looks okay.
	! [ -e "$ROOTFS/etc" ]
	! [ -e "$ROOTFS/bin/sh" ]
	! [ -e "$ROOTFS/usr/bin/env" ]

	# Make sure that the new layer is a non-empty_layer.
	umoci stat --image "${IMAGE}:${TAG}-new" --json
	[ "$status" -eq 0 ]
	[[ "$(echo "$output" | jq -SM '.history[-1].empty_layer')" == "null" ]]

	# Try to create a layer containing a ".wh." file.
	mkdir -p "$ROOTFS/whiteout_test"
	echo "some data" > "$ROOTFS/whiteout_test/.wh. THIS IS A TEST "

	# Repacking a rootfs with a '.wh.' file *must* fail.
	umoci repack --image "${IMAGE}:${TAG}-new2" "$BUNDLE"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Try to create a layer containing a ".wh." directory.
	rm_rf "$ROOTFS/whiteout_test"
	mkdir -p "$ROOTFS/.wh.another_whiteout"

	# Repacking a rootfs with a '.wh.' directory *must* fail.
	umoci repack --image "${IMAGE}:${TAG}-new3" "$BUNDLE"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"
}

@test "umoci repack [replace]" {
	# Unpack the image.
	new_bundle_rootfs && ROOTFS_A="$ROOTFS"
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	# Make sure the files we're replacing exist.
	[ -d "$ROOTFS/etc" ]
	[ -L "$ROOTFS/bin/sh" ]
	[ -e "$ROOTFS/usr/bin/env" ]

	# Replace them.
	rm_rf "$ROOTFS/"{etc,bin/sh,usr/bin/env}
	echo "different" > "$ROOTFS/etc"
	mkdir "$ROOTFS/bin/sh"
	ln -s "a \\really //weird _00..:=path " "$ROOTFS/usr/bin/env"

	# Repack the image under the same tag.
	umoci repack --image "${IMAGE}:${TAG}" "$BUNDLE"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Unpack it again.
	new_bundle_rootfs && BUNDLE_B="$BUNDLE"
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	# Ensure that gomtree succeeds on the old bundle, which is what this was
	# generated from.
	gomtree -p "$ROOTFS_A" -f "$BUNDLE_B"/sha256_*.mtree
	[ "$status" -eq 0 ]
	[ -z "$output" ]

	# Just for sanity, check that everything looks okay.
	[ -f "$ROOTFS/etc" ]
	[ -d "$ROOTFS/bin/sh" ]
	[ -L "$ROOTFS/usr/bin/env" ]

	# Make sure that the new layer is a non-empty_layer.
	umoci stat --image "${IMAGE}:${TAG}" --json
	[ "$status" -eq 0 ]
	[[ "$(echo "$output" | jq -SM '.history[-1].empty_layer')" == "null" ]]

	image-verify "${IMAGE}"
}

@test "umoci repack --history.*" {
	# Unpack the image.
	new_bundle_rootfs
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	# Make some small change.
	touch "$ROOTFS/a_small_change"
	now="$(date --iso-8601=seconds --utc)"

	# Repack the image, setting history values.
	umoci repack --image "${IMAGE}:${TAG}-new" \
		--history.author="Some Author <jane@blogg.com>" \
		--history.comment="Made a_small_change." \
		--history.created_by="touch '$BUNDLE/a_small_change'" \
		--history.created="$now" "$BUNDLE"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Make sure that the history was modified.
	umoci stat --image "${IMAGE}:${TAG}" --json
	[ "$status" -eq 0 ]
	numLinesA="$(echo "$output" | jq -SMr '.history | length')"

	umoci stat --image "${IMAGE}:${TAG}-new" --json
	[ "$status" -eq 0 ]
	numLinesB="$(echo "$output" | jq -SMr '.history | length')"

	# Number of lines should be greater.
	[ "$numLinesB" -gt "$numLinesA" ]
	# The final layer should not be an empty_layer now.
	[[ "$(echo "$output" | jq -SMr '.history[-1].empty_layer')" == "null" ]]
	# The author should've changed to --history.author.
	[[ "$(echo "$output" | jq -SMr '.history[-1].author')" == "Some Author <jane@blogg.com>" ]]
	# The comment should be added.
	[[ "$(echo "$output" | jq -SMr '.history[-1].comment')" == "Made a_small_change." ]]
	# The created_by should be set.
	[[ "$(echo "$output" | jq -SMr '.history[-1].created_by')" == "touch '$BUNDLE/a_small_change'" ]]
	# The created should be set.
	[[ "$(date --iso-8601=seconds --utc --date="$(echo "$output" | jq -SMr '.history[-1].created')")" == "$now" ]]

	image-verify "${IMAGE}"
}

@test "umoci repack --no-history" {
	# Unpack the image.
	new_bundle_rootfs
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	# Create some file.
	echo "foo" > "$ROOTFS/foobar"

	# Repack the image under a new tag, but with no history change.
	umoci repack --no-history --image "${IMAGE}:${TAG}-new" "$BUNDLE"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Make sure we *did not* add a new history entry.
	umoci stat --image "${IMAGE}:${TAG}" --json
	[ "$status" -eq 0 ]
	hashA="$(jq '.history' <<<"$output" | sha256sum)"

	umoci stat --image "${IMAGE}:${TAG}-new" --json
	[ "$status" -eq 0 ]
	hashB="$(jq '.history' <<<"$output" | sha256sum)"

	# umoci-stat history output should be identical.
	[[ "$hashA" == "$hashB" ]]

	image-verify "${IMAGE}"
}

@test "umoci {un,re}pack [hardlink]" {
	# Unpack the image.
	new_bundle_rootfs
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	# Create a file and some hardlinks.
	echo "this has some contents" >> "$ROOTFS/small_change"
	ln -f "$ROOTFS/small_change" "$ROOTFS/link_hard"
	mkdir -p "$ROOTFS/tmp" && ln -f "$ROOTFS/small_change" "$ROOTFS/tmp/link_hard"
	mkdir -p "$ROOTFS/another/link/dir" && ln -f "$ROOTFS/link_hard" "$ROOTFS/another/link/dir/hard"

	# Symlink + hardlink.
	ln -sf "/../../.././small_change" "$ROOTFS/symlink"
	ln -Pf "$ROOTFS/symlink" "$ROOTFS/tmp/symlink_hard"

	# Repack the image, setting history values.
	umoci repack --image "${IMAGE}:${TAG}-new" "$BUNDLE"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Unpack the image again.
	new_bundle_rootfs
	umoci unpack --image "${IMAGE}:${TAG}-new" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	# Now make sure that the paths all have the same inode numbers.
	sane_run stat -c 'ino=%i nlink=%h type=%f' "$ROOTFS/small_change"
	[ "$status" -eq 0 ]
	originalA="$output"
	sane_run stat -c 'ino=%i nlink=%h type=%f' "$ROOTFS/link_hard"
	[ "$status" -eq 0 ]
	[[ "$output" == "$originalA" ]]
	sane_run stat -c 'ino=%i nlink=%h type=%f' "$ROOTFS/tmp/link_hard"
	[ "$status" -eq 0 ]
	[[ "$output" == "$originalA" ]]
	sane_run stat -c 'ino=%i nlink=%h type=%f' "$ROOTFS/another/link/dir/hard"
	[ "$status" -eq 0 ]
	[[ "$output" == "$originalA" ]]

	# Now make sure that the paths all have the same inode numbers.
	sane_run stat -c 'ino=%i nlink=%h type=%f' "$ROOTFS/symlink"
	[ "$status" -eq 0 ]
	originalB="$output"
	sane_run stat -c 'ino=%i nlink=%h type=%f' "$ROOTFS/tmp/symlink_hard"
	[ "$status" -eq 0 ]
	[[ "$output" == "$originalB" ]]

	# Make sure that hardlink->symlink != hardlink.
	[[ "$originalA" != "$originalB" ]]

	image-verify "${IMAGE}"
}

@test "umoci {un,re}pack [unpriv]" {
	# Unpack the image.
	new_bundle_rootfs
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	# Create some directories for unpriv check.
	mkdir -p "$ROOTFS/some/directory/path"

	# mkfifo and some other stuff
	mkfifo "$ROOTFS/some/directory/path/fifo"
	echo "some contents" > "$ROOTFS/some/directory/path/file"
	mkdir "$ROOTFS/some/directory/path/dir"
	ln -s "/../././././/../../../../etc/shadow" "$ROOTFS/some/directory/path/link"

	# Make sure that replacing a file we don't have write access to works.
	echo "another file" > "$ROOTFS/some/directory/NOWRITE"
	chmod 0000 "$ROOTFS/some/directory/NOWRITE"

	# Chmod.
	chmod 0000 "$ROOTFS/some/directory/path"
	chmod 0000 "$ROOTFS/some/directory"
	chmod 0000 "$ROOTFS/some"

	# Repack the image.
	umoci repack --image "${IMAGE}" "$BUNDLE"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Unpack the image again.
	new_bundle_rootfs
	umoci unpack --image "${IMAGE}" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	# Undo the chmodding.
	chmod +rwx "$ROOTFS/some"
	chmod +rwx "$ROOTFS/some/directory"
	chmod +rwx "$ROOTFS/some/directory/path"
	chmod +rwx "$ROOTFS/some/directory/NOWRITE"

	# Make sure the types are right.
	[[ "$(stat -c '%F' "$ROOTFS/some/directory/path/fifo")" == "fifo" ]]
	[[ "$(stat -c '%F' "$ROOTFS/some/directory/path/file")" == "regular file" ]]
	[ -f "$ROOTFS/some/directory/NOWRITE" ]
	[[ "$(stat -c '%F' "$ROOTFS/some/directory/path/dir")" == "directory" ]]
	[[ "$(stat -c '%F' "$ROOTFS/some/directory/path/link")" == "symbolic link" ]]

	# Try to overwite the NOWRITE file.
	echo "different data" > "$ROOTFS/some/directory/NOWRITE"
	chmod 0000 "$ROOTFS/some/directory/NOWRITE"

	# Chmod.
	chmod 0000 "$ROOTFS/some/directory/path"
	chmod 0000 "$ROOTFS/some/directory"
	chmod 0000 "$ROOTFS/some"

	# Repack the image again.
	umoci repack --image "${IMAGE}" "$BUNDLE"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Unpack the image again.
	new_bundle_rootfs
	umoci unpack --image "${IMAGE}" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	# Undo the chmodding.
	chmod +rwx "$ROOTFS/some"
	chmod +rwx "$ROOTFS/some/directory"
	chmod +rwx "$ROOTFS/some/directory/path"
	chmod +rwx "$ROOTFS/some/directory/NOWRITE"

	# Check NOWRITE.
	[ -f "$ROOTFS/some/directory/NOWRITE" ]
	[[ "$(cat "$ROOTFS/some/directory/NOWRITE")" == "different data" ]]

	image-verify "${IMAGE}"
}

@test "umoci {un,re}pack [xattrs]" {
	# Unpack the image.
	new_bundle_rootfs
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	# Make a test directory to make sure nesting works.
	mkdir -p "$ROOTFS/some/test/directory"
	xattr -w user.toplevel.some "some directory" "$ROOTFS/some"
	xattr -w user.midlevel.test "test directory" "$ROOTFS/some/test"
	xattr -w user.lowlevel.direct "directory" "$ROOTFS/some/test/directory"

	# Set user.* xattrs.
	chmod +w "$ROOTFS/root" && xattr -w user.some.value thisisacoolfile	"$ROOTFS/root"
	chmod +w "$ROOTFS/etc"  && xattr -w user.another	valuegoeshere	  "$ROOTFS/etc"
	chmod +w "$ROOTFS/var"  && xattr -w user.3rd		halflife3confirmed "$ROOTFS/var"
	chmod +w "$ROOTFS/usr"  && xattr -w user."key also" "works if you try" "$ROOTFS/usr"
	chmod +w "$ROOTFS/lib"  && xattr -w user.empty_cont ""				 "$ROOTFS/lib"
	# Forbidden xattr.
	chmod +w "$ROOTFS/opt"  && xattr -w "user.UMOCI:forbidden_xattr" "should not exist" "$ROOTFS/opt"

	# Repack the image.
	umoci repack --image "${IMAGE}" "$BUNDLE"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Unpack the image again.
	new_bundle_rootfs
	umoci unpack --image "${IMAGE}" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	# Make sure the xattrs have been set.
	sane_run xattr -p user.toplevel.some "$ROOTFS/some"
	[ "$status" -eq 0 ]
	[[ "$output" == "some directory" ]]
	sane_run xattr -p user.midlevel.test "$ROOTFS/some/test"
	[ "$status" -eq 0 ]
	[[ "$output" == "test directory" ]]
	sane_run xattr -p user.lowlevel.direct "$ROOTFS/some/test/directory"
	[ "$status" -eq 0 ]
	[[ "$output" == "directory" ]]
	sane_run xattr -p user.some.value "$ROOTFS/root"
	[ "$status" -eq 0 ]
	[[ "$output" == "thisisacoolfile" ]]
	sane_run xattr -p user.another "$ROOTFS/etc"
	[ "$status" -eq 0 ]
	[[ "$output" == "valuegoeshere" ]]
	sane_run xattr -p user.3rd "$ROOTFS/var"
	[ "$status" -eq 0 ]
	[[ "$output" == "halflife3confirmed" ]]
	sane_run xattr -p user."key also" "$ROOTFS/usr"
	[ "$status" -eq 0 ]
	[[ "$output" == "works if you try" ]]
	# Empty-valued xattrs are disallowed by PAX.
	sane_run xattr -p user.empty_cont "$ROOTFS/lib"
	[[ "$output" == *"No such xattr: user.empty_cont"* ]]
	# Forbidden xattrs are ignored.
	sane_run xattr -p "user.UMOCI:forbidden_xattr" "$ROOTFS/opt"
	[[ "$output" == *"No such xattr: user.UMOCI:forbidden_xattr"* ]]

	# Now make some changes.
	xattr -d user.some.value "$ROOTFS/root"
	xattr -d user.midlevel.test "$ROOTFS/some/test"
	xattr -w user.3rd "jk, hl3 isn't here yet" "$ROOTFS/var"

	# Repack the image.
	umoci repack --image "${IMAGE}" "$BUNDLE"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Unpack the image again.
	new_bundle_rootfs
	umoci unpack --image "${IMAGE}" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	# Make sure the xattrs have been set.
	sane_run xattr -p user.toplevel.some "$ROOTFS/some"
	[ "$status" -eq 0 ]
	[[ "$output" == "some directory" ]]
	sane_run xattr -p user.midlevel.test "$ROOTFS/some/test"
	[[ "$output" == *"No such xattr: user.midlevel.test"* ]]
	sane_run xattr -p user.lowlevel.direct "$ROOTFS/some/test/directory"
	[ "$status" -eq 0 ]
	[[ "$output" == "directory" ]]
	sane_run xattr -p user.some.value "$ROOTFS/root"
	[[ "$output" == *"No such xattr: user.some.value"* ]]
	sane_run xattr -p user.another "$ROOTFS/etc"
	[ "$status" -eq 0 ]
	[[ "$output" == "valuegoeshere" ]]
	sane_run xattr -p user.3rd "$ROOTFS/var"
	[ "$status" -eq 0 ]
	[[ "$output" == "jk, hl3 isn't here yet" ]]
	sane_run xattr -p user."key also" "$ROOTFS/usr"
	[ "$status" -eq 0 ]
	[[ "$output" == "works if you try" ]]
	# Empty-valued xattrs are disallowed by PAX.
	sane_run xattr -p user.empty_cont "$ROOTFS/lib"
	[[ "$output" == *"No such xattr: user.empty_cont"* ]]
	# Forbidden xattrs are ignored.
	sane_run xattr -p "user.UMOCI:forbidden_xattr" "$ROOTFS/opt"
	[[ "$output" == *"No such xattr: user.UMOCI:forbidden_xattr"* ]]

	image-verify "${IMAGE}"
}

@test "umoci {un,re}pack [unicode]" {
	# Unpack the image.
	new_bundle_rootfs
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	# Unicode is very fun.
	mkdir "$ROOTFS/TÜBİTAK_UEKAE_Kök_Sertifika_ Hizmet Sağlayıcısı -_Sürüm_3"
	touch "$ROOTFS/TÜBİTAK_UEKAE_Kök_Sertifika_ Hizmet Sağlayıcısı -_Sürüm_3/NetLock_Arany_=Class_Gold=_Főtanúsítvány.pem"
	touch "$ROOTFS/AC_Raíz_Certicámara_S.A..pem"
	touch "$ROOTFS/ <-- some more weird characters --> 你好，世界"
	touch "$ROOTFS/変な字じゃないけど色んなソフトは全然処理できないんだよ。。。"

	# Repack the image.
	umoci repack --image "${IMAGE}" "$BUNDLE"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Unpack the image again.
	new_bundle_rootfs
	umoci unpack --image "${IMAGE}" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	# Make sure the directories and files exist.
	[ -d "$ROOTFS/TÜBİTAK_UEKAE_Kök_Sertifika_ Hizmet Sağlayıcısı -_Sürüm_3" ]
	[ -f "$ROOTFS/TÜBİTAK_UEKAE_Kök_Sertifika_ Hizmet Sağlayıcısı -_Sürüm_3/NetLock_Arany_=Class_Gold=_Főtanúsítvány.pem" ]
	[ -f "$ROOTFS/AC_Raíz_Certicámara_S.A..pem" ]
	[ -f "$ROOTFS/ <-- some more weird characters --> 你好，世界" ]
	[ -f "$ROOTFS/変な字じゃないけど色んなソフトは全然処理できないんだよ。。。" ]

	# Now make some changes.
	rm_rf "$ROOTFS/AC_Raíz_Certicámara_S.A..pem"

	# Repack the image.
	umoci repack --image "${IMAGE}" "$BUNDLE"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Unpack the image again.
	new_bundle_rootfs
	umoci unpack --image "${IMAGE}" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	# Make sure the directories and files exist.
	[ -d "$ROOTFS/TÜBİTAK_UEKAE_Kök_Sertifika_ Hizmet Sağlayıcısı -_Sürüm_3" ]
	[ -f "$ROOTFS/TÜBİTAK_UEKAE_Kök_Sertifika_ Hizmet Sağlayıcısı -_Sürüm_3/NetLock_Arany_=Class_Gold=_Főtanúsítvány.pem" ]
	! [ -f "$ROOTFS/AC_Raíz_Certicámara_S.A..pem" ]
	[ -f "$ROOTFS/ <-- some more weird characters --> 你好，世界" ]

	image-verify "${IMAGE}"
}

@test "umoci repack [volumes]" {
	# Set some paths to be volumes.
	umoci config --image "${IMAGE}:${TAG}" --config.volume /volume --config.volume "/some nutty/path name/ here"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Unpack the image.
	new_bundle_rootfs && BUNDLE_A="$BUNDLE"
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	# Create files in those volumes.
	mkdir -p "$ROOTFS/some nutty/path name/"
	echo "this is a test" > "$ROOTFS/some nutty/path name/ here"
	mkdir -p "$ROOTFS/volume"
	echo "another test" > "$ROOTFS/volume/test"
	# ... and some outside.
	echo "more tests" > "$ROOTFS/some nutty/path "
	mkdir -p "$ROOTFS/some/volume"
	echo "in a mirror directory" > "$ROOTFS/some/volume/test"
	echo "checking mirror" > "$ROOTFS/volumetest"

	# Repack the image under a new tag.
	umoci repack --image "${IMAGE}:${TAG}-new" "$BUNDLE"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Re-extract to verify the volume changes weren't included.
	new_bundle_rootfs
	umoci unpack --image "${IMAGE}:${TAG}-new" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	# Check the files.
	[ -f "$ROOTFS/some nutty/path " ]
	[[ "$(cat "$ROOTFS/some nutty/path ")" == "more tests" ]]
	[ -d "$ROOTFS/some/volume" ]
	[ -f "$ROOTFS/some/volume/test" ]
	[[ "$(cat "$ROOTFS/some/volume/test")" == "in a mirror directory" ]]
	[ -f "$ROOTFS/volumetest" ]
	[[ "$(cat "$ROOTFS/volumetest")" == "checking mirror" ]]

	# Volume paths must not be included.
	! [ -e "$ROOTFS/volume" ]
	! [ -e "$ROOTFS/volume/test" ]
	! [ -e "$ROOTFS/some nutty/path name" ]
	! [ -e "$ROOTFS/some nutty/path name/ here" ]

	# Repack a copy of the original with volumes not masked.
	umoci repack --image "${IMAGE}:${TAG}-nomask" --no-mask-volumes "$BUNDLE_A"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Extract the no-mask variant to make sure that masked changes *were* included.
	new_bundle_rootfs
	umoci unpack --image "${IMAGE}:${TAG}-nomask" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	# Check the files.
	[ -f "$ROOTFS/some nutty/path " ]
	[[ "$(cat "$ROOTFS/some nutty/path ")" == "more tests" ]]
	[ -d "$ROOTFS/some/volume" ]
	[ -f "$ROOTFS/some/volume/test" ]
	[[ "$(cat "$ROOTFS/some/volume/test")" == "in a mirror directory" ]]
	[ -f "$ROOTFS/volumetest" ]
	[[ "$(cat "$ROOTFS/volumetest")" == "checking mirror" ]]

	# Volume paths must be included, as well as their contents.
	[ -e "$ROOTFS/volume" ]
	[ -f "$ROOTFS/volume/test" ]
	[[ "$(cat "$ROOTFS/volume/test")" == "another test" ]]
	[ -d "$ROOTFS/some nutty/path name" ]
	[ -f "$ROOTFS/some nutty/path name/ here" ]
	[[ "$(cat "$ROOTFS/some nutty/path name/ here")" == "this is a test" ]]

	# Re-do everything but this time with --mask-path.
	umoci repack --image "${IMAGE}:${TAG}-new" --mask-path /volume "$BUNDLE_A"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Re-extract to verify the masked path changes weren't included.
	new_bundle_rootfs
	umoci unpack --image "${IMAGE}:${TAG}-new" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	# Check the files.
	[ -f "$ROOTFS/some nutty/path " ]
	[[ "$(cat "$ROOTFS/some nutty/path ")" == "more tests" ]]
	[ -d "$ROOTFS/some/volume" ]
	[ -f "$ROOTFS/some/volume/test" ]
	[[ "$(cat "$ROOTFS/some/volume/test")" == "in a mirror directory" ]]
	[ -f "$ROOTFS/volumetest" ]
	[[ "$(cat "$ROOTFS/volumetest")" == "checking mirror" ]]

	# Masked paths must not be included.
	! [ -e "$ROOTFS/volume" ]
	! [ -e "$ROOTFS/volume/test" ]
	# And volumes will also not be included.
	! [ -e "$ROOTFS/some nutty/path name" ]
	! [ -e "$ROOTFS/some nutty/path name/ here" ]
}

@test "umoci repack --refresh-bundle" {
	# Unpack the original image
	new_bundle_rootfs && BUNDLE_A="$BUNDLE" ROOTFS_A="$ROOTFS"
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	# Make sure the files we're creating don't exist.
	! [ -e "$ROOTFS/newfile" ]
	! [ -e "$ROOTFS/newdir" ]
	! [ -e "$ROOTFS/newdir/anotherfile" ]

	# Create them.
	echo "first file" > "$ROOTFS/newfile"
	mkdir "$ROOTFS/newdir"
	echo "subfile" > "$ROOTFS/newdir/anotherfile"

	# Repack the image under a new tag, refreshing the bundle metadata.
	umoci repack --refresh-bundle --image "${IMAGE}:${TAG}-new" "$BUNDLE"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Ensure the gomtree has been refreshed in the bundle
	gomtree -p "$ROOTFS" -f "$BUNDLE"/sha256_*.mtree
	[ "$status" -eq 0 ]
	[ -z "$output" ]

	# Unpack it again.
	new_bundle_rootfs && BUNDLE_B="$BUNDLE" ROOTFS_B="$ROOTFS"
	umoci unpack --image "${IMAGE}:${TAG}-new" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	# Ensure that gomtree succeeds across bundles - they should be the same rootfs
	# and have the same mtree manifest
	gomtree -p "$ROOTFS_A" -f "$BUNDLE_B"/sha256_*.mtree
	[ "$status" -eq 0 ]
	[ -z "$output" ]
	gomtree -p "$ROOTFS_B" -f "$BUNDLE_A"/sha256_*.mtree
	[ "$status" -eq 0 ]
	[ -z "$output" ]

	# Make some other changes to the first bundle
	echo "second file" > "$ROOTFS_A/newfile2"

	# Repack under a new tag again, without refreshing the metadata.
	umoci repack --image "${IMAGE}:${TAG}-new2" "$BUNDLE_A"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Unpack it again into a new bundle.
	new_bundle_rootfs && BUNDLE_C="$BUNDLE" ROOTFS_C="$ROOTFS"
	umoci unpack --image "${IMAGE}:${TAG}-new2" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	# Ensure all changes are reflected
	gomtree -p "$ROOTFS_A" -f "$BUNDLE_C"/sha256_*.mtree
	[ "$status" -eq 0 ]
	[ -z "$output" ]
	gomtree -p "$ROOTFS_C" -f "$BUNDLE_C"/sha256_*.mtree
	[ "$status" -eq 0 ]
	[ -z "$output" ]

	# Final bundle sanity check
	[ -f "$ROOTFS/newfile" ]
	[ -d "$ROOTFS/newdir" ]
	[ -f "$ROOTFS/newdir/anotherfile" ]
	[ -f "$ROOTFS/newfile2" ]

	# Now check the image.
	# Make sure we added a new layer on both repacks.
	umoci stat --image "${IMAGE}:${TAG}" --json
	[ "$status" -eq 0 ]
	numLinesA="$(echo "$output" | jq -SM '.history | length')"

	umoci stat --image "${IMAGE}:${TAG}-new" --json
	[ "$status" -eq 0 ]
	numLinesB="$(echo "$output" | jq -SM '.history | length')"

	umoci stat --image "${IMAGE}:${TAG}-new2" --json
	[ "$status" -eq 0 ]
	numLinesC="$(echo "$output" | jq -SM '.history | length')"

	# Number of lines should be greater.
	[ "$numLinesB" -gt "$numLinesA" ]
	[ "$numLinesC" -gt "$numLinesB" ]
}

@test "umoci repack (empty diff)" {
	# Unpack the original image
	new_bundle_rootfs
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	# Repack the image under a new tag.
	umoci repack --image "${IMAGE}:${TAG}-new" "$BUNDLE"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# The two manifests should have the same number of layers.
	manifest0=$(cat "${IMAGE}/oci/index.json" | jq -r .manifests[0].digest | cut -f2 -d:)
	manifest1=$(cat "${IMAGE}/oci/index.json" | jq -r .manifests[1].digest | cut -f2 -d:)

	layers0=$(cat "${IMAGE}/oci/blobs/sha256/$manifest0" | jq -r .layers)
	layers1=$(cat "${IMAGE}/oci/blobs/sha256/$manifest1" | jq -r .layers)
	[ "$layers0" == "$layers1" ]
}

OCI_MEDIATYPE_LAYER="application/vnd.oci.image.layer.v1.tar"

@test "umoci repack --compress=gzip" {
	# Unpack the image.
	new_bundle_rootfs
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	# Make sure we make a new tar layer.
	touch "$ROOTFS/new-file"
	# Add layer to the image.
	umoci repack --image "${IMAGE}:${TAG}" --compress=gzip "$BUNDLE"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	umoci stat --image "${IMAGE}:${TAG}" --json
	[ "$status" -eq 0 ]
	stat_json="$output"

	# Make sure that the last layer had the expected compression based on the
	# mediatype.
	expected_mediatype="${OCI_MEDIATYPE_LAYER}+gzip"
	layer_mediatype="$(jq -SMr '.history[-1].layer.mediaType' <<<"$stat_json")"
	[[ "$layer_mediatype" == "$expected_mediatype" ]]

	# Make sure that the actual blob seems to be a gzip
	layer_hash="$(jq -SMr '.history[-1].layer.digest' <<<"$stat_json" | tr : /)"
	sane_run file -i "$IMAGE/blobs/$layer_hash"
	[ "$status" -eq 0 ]
	[[ "$output" == *"application/gzip"* ]]
}

@test "umoci repack --compress=zstd" {
	# Unpack the image.
	new_bundle_rootfs
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	# Make sure we make a new tar layer.
	touch "$ROOTFS/new-file"
	# Add layer to the image.
	umoci repack --image "${IMAGE}:${TAG}" --compress=zstd "$BUNDLE"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	umoci stat --image "${IMAGE}:${TAG}" --json
	[ "$status" -eq 0 ]
	stat_json="$output"

	# Make sure that the last layer had the expected compression based on the
	# mediatype.
	expected_mediatype="${OCI_MEDIATYPE_LAYER}+zstd"
	layer_mediatype="$(jq -SMr '.history[-1].layer.mediaType' <<<"$stat_json")"
	[[ "$layer_mediatype" == "$expected_mediatype" ]]

	# Make sure that the actual blob seems to be a gzip
	layer_hash="$(jq -SMr '.history[-1].layer.digest' <<<"$stat_json" | tr : /)"
	sane_run file -i "$IMAGE/blobs/$layer_hash"
	[ "$status" -eq 0 ]
	[[ "$output" == *"application/zstd"* ]]
}

@test "umoci repack --compress=none" {
	# Unpack the image.
	new_bundle_rootfs
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	# Make sure we make a new tar layer.
	touch "$ROOTFS/new-file"
	# Add layer to the image.
	umoci repack --image "${IMAGE}:${TAG}" --compress=none "$BUNDLE"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	umoci stat --image "${IMAGE}:${TAG}" --json
	[ "$status" -eq 0 ]
	stat_json="$output"

	# Make sure that the last layer had the expected compression based on the
	# mediatype.
	expected_mediatype="${OCI_MEDIATYPE_LAYER}"
	layer_mediatype="$(jq -SMr '.history[-1].layer.mediaType' <<<"$stat_json")"
	[[ "$layer_mediatype" == "$expected_mediatype" ]]

	# Make sure that the actual blob seems to be a gzip
	layer_hash="$(jq -SMr '.history[-1].layer.digest' <<<"$stat_json" | tr : /)"
	sane_run file -i "$IMAGE/blobs/$layer_hash"
	[ "$status" -eq 0 ]
	[[ "$output" == *"application/x-tar"* ]] # x-tar means no compression
}

@test "umoci repack --refresh-bundle --compress=auto" {
	# Unpack the image.
	new_bundle_rootfs
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	# Make sure we make a new tar layer.
	touch "$ROOTFS/new-file1"
	# Add zstd layer to the image.
	umoci repack --image "${IMAGE}:${TAG}" --refresh-bundle --compress=zstd "$BUNDLE"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	umoci stat --image "${IMAGE}:${TAG}" --json
	[ "$status" -eq 0 ]
	stat_json="$output"

	# Make sure that the last layer had the expected compression based on the
	# mediatype.
	expected_mediatype="${OCI_MEDIATYPE_LAYER}+zstd"
	layer_mediatype="$(jq -SMr '.history[-1].layer.mediaType' <<<"$stat_json")"
	[[ "$layer_mediatype" == "$expected_mediatype" ]]

	# Make sure that the actual blob seems to be a gzip
	layer_hash="$(jq -SMr '.history[-1].layer.digest' <<<"$stat_json" | tr : /)"
	sane_run file -i "$IMAGE/blobs/$layer_hash"
	[ "$status" -eq 0 ]
	[[ "$output" == *"application/zstd"* ]]

	# Make sure we make a new tar layer.
	touch "$ROOTFS/new-file2"
	# Add another zstd layer to the image, by making use of the auto selection.
	umoci repack --image "${IMAGE}:${TAG}" --refresh-bundle --compress=auto "$BUNDLE"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	umoci stat --image "${IMAGE}:${TAG}" --json
	[ "$status" -eq 0 ]
	stat_json="$output"

	# Make sure that the last layer had the expected compression based on the
	# mediatype.
	expected_mediatype="${OCI_MEDIATYPE_LAYER}+zstd"
	layer_mediatype="$(jq -SMr '.history[-1].layer.mediaType' <<<"$stat_json")"
	[[ "$layer_mediatype" == "$expected_mediatype" ]]

	# Make sure that the actual blob seems to be a gzip
	layer_hash="$(jq -SMr '.history[-1].layer.digest' <<<"$stat_json" | tr : /)"
	sane_run file -i "$IMAGE/blobs/$layer_hash"
	[ "$status" -eq 0 ]
	[[ "$output" == *"application/zstd"* ]]

	# Make sure we make a new tar layer.
	touch "$ROOTFS/new-file3"
	# Add yet another zstd layer to the image, to show that --compress=auto is
	# the default.
	umoci repack --image "${IMAGE}:${TAG}" --refresh-bundle "$BUNDLE"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	umoci stat --image "${IMAGE}:${TAG}" --json
	[ "$status" -eq 0 ]
	stat_json="$output"

	# Make sure that the last layer had the expected compression based on the
	# mediatype.
	expected_mediatype="${OCI_MEDIATYPE_LAYER}+zstd"
	layer_mediatype="$(jq -SMr '.history[-1].layer.mediaType' <<<"$stat_json")"
	[[ "$layer_mediatype" == "$expected_mediatype" ]]

	# Make sure that the actual blob seems to be a gzip
	layer_hash="$(jq -SMr '.history[-1].layer.digest' <<<"$stat_json" | tr : /)"
	sane_run file -i "$IMAGE/blobs/$layer_hash"
	[ "$status" -eq 0 ]
	[[ "$output" == *"application/zstd"* ]]
}

@test "umoci repack [cas file ownership]" {
	requires root

	# Change the image ownership to a random uid:gid.
	chown -R 1234:5678 "$IMAGE"
	image-verify "$IMAGE"

	# Unpack the image.
	new_bundle_rootfs
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	# Make some changes.
	touch "$ROOTFS/etc"
	echo "first file" > "$ROOTFS/newfile"
	mkdir "$ROOTFS/newdir"
	echo "subfile" > "$ROOTFS/newdir/anotherfile"
	ln -s "this is a dummy symlink" "$ROOTFS/newdir/link"

	umoci repack --image "${IMAGE}:${TAG}" --refresh-bundle "$BUNDLE"
	[ "$status" -eq 0 ]
	image-verify "$IMAGE"

	# image-verify checks that the ownership is correct, but double-check
	# explicitly that all of the files are owned by the user we expected.
	sane_run bats_pipe find "$IMAGE" -print0 \| xargs -0 stat -c "%u:%g %n" \| grep -v "^1234:5678 "
	[ "$status" -ne 0 ]
	[ -z "$output" ]
}
