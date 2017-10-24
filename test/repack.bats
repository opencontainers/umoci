#!/usr/bin/env bats -t
# umoci: Umoci Modifies Open Containers' Images
# Copyright (C) 2016, 2017 SUSE LLC.
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
	setup_image
}

function teardown() {
	teardown_tmpdirs
	teardown_image
}

@test "umoci repack" {
	BUNDLE_A="$(setup_tmpdir)"
	BUNDLE_B="$(setup_tmpdir)"

	image-verify "${IMAGE}"

	# Unpack the image.
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE_A"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE_A"

	# Make sure the files we're creating don't exist.
	! [ -e "$BUNDLE_A/rootfs/newfile" ]
	! [ -e "$BUNDLE_A/rootfs/newdir" ]
	! [ -e "$BUNDLE_A/rootfs/newdir/anotherfile" ]
	! [ -e "$BUNDLE_A/rootfs/newdir/link" ]

	# Create them.
	echo "first file" > "$BUNDLE_A/rootfs/newfile"
	mkdir "$BUNDLE_A/rootfs/newdir"
	echo "subfile" > "$BUNDLE_A/rootfs/newdir/anotherfile"
	ln -s "this is a dummy symlink" "$BUNDLE_A/rootfs/newdir/link"

	# Repack the image under a new tag.
	umoci repack --image "${IMAGE}:${TAG}-new" "$BUNDLE_A"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Unpack it again.
	umoci unpack --image "${IMAGE}:${TAG}-new" "$BUNDLE_B"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE_B"

	# Ensure that gomtree succeeds on the old bundle, which is what this was
	# generated from.
	gomtree -p "$BUNDLE_A/rootfs" -f "$BUNDLE_B"/sha256_*.mtree
	[ "$status" -eq 0 ]
	[ -z "$output" ]

	# Just for sanity, check that everything looks okay.
	[ -f "$BUNDLE_B/rootfs/newfile" ]
	[ -d "$BUNDLE_B/rootfs/newdir" ]
	[ -f "$BUNDLE_B/rootfs/newdir/anotherfile" ]
	[ -L "$BUNDLE_B/rootfs/newdir/link" ]

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

@test "umoci repack [missing args]" {
	BUNDLE="$(setup_tmpdir)"

	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE"
	[ "$status" -eq 0 ]

	umoci repack --image "${IMAGE}:${TAG}"
	[ "$status" -ne 0 ]

	umoci repack "$BUNDLE"
	[ "$status" -ne 0 ]
}

@test "umoci repack [whiteout]" {
	BUNDLE_A="$(setup_tmpdir)"
	BUNDLE_B="$(setup_tmpdir)"

	image-verify "${IMAGE}"

	# Unpack the image.
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE_A"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE_A"

	# Make sure the files we're deleting exist.
	[ -d "$BUNDLE_A/rootfs/etc" ]
	[ -L "$BUNDLE_A/rootfs/bin/sh" ]
	[ -e "$BUNDLE_A/rootfs/usr/bin/env" ]

	# Remove them.
	chmod +w "$BUNDLE_A/rootfs/etc/." && rm -rf "$BUNDLE_A/rootfs/etc"
	chmod +w "$BUNDLE_A/rootfs/bin/." && rm "$BUNDLE_A/rootfs/bin/sh"
	chmod +w "$BUNDLE_A/rootfs/usr/bin/." && rm "$BUNDLE_A/rootfs/usr/bin/env"

	# Repack the image under a new tag.
	umoci repack --image "${IMAGE}:${TAG}-new" "$BUNDLE_A"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Unpack it again.
	umoci unpack --image "${IMAGE}:${TAG}-new" "$BUNDLE_B"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE_B"

	# Ensure that gomtree suceeds on the old bundle, which is what this was
	# generated from.
	gomtree -p "$BUNDLE_A/rootfs" -f "$BUNDLE_B"/sha256_*.mtree
	[ "$status" -eq 0 ]
	[ -z "$output" ]

	# Just for sanity, check that everything looks okay.
	! [ -e "$BUNDLE_A/rootfs/etc" ]
	! [ -e "$BUNDLE_A/rootfs/bin/sh" ]
	! [ -e "$BUNDLE_A/rootfs/usr/bin/env" ]

	# Make sure that the new layer is a non-empty_layer.
	umoci stat --image "${IMAGE}:${TAG}-new" --json
	[ "$status" -eq 0 ]
	[[ "$(echo "$output" | jq -SM '.history[-1].empty_layer')" == "null" ]]
}

@test "umoci repack [replace]" {
	BUNDLE_A="$(setup_tmpdir)"
	BUNDLE_B="$(setup_tmpdir)"

	image-verify "${IMAGE}"

	# Unpack the image.
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE_A"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE_A"

	# Make sure the files we're replacing exist.
	[ -d "$BUNDLE_A/rootfs/etc" ]
	[ -L "$BUNDLE_A/rootfs/bin/sh" ]
	[ -e "$BUNDLE_A/rootfs/usr/bin/env" ]

	# Replace them.
	chmod +w "$BUNDLE_A/rootfs/etc/." && rm -rf "$BUNDLE_A/rootfs/etc"
	echo "different" > "$BUNDLE_A/rootfs/etc"
	chmod +w "$BUNDLE_A/rootfs/bin/." && rm "$BUNDLE_A/rootfs/bin/sh"
	mkdir "$BUNDLE_A/rootfs/bin/sh"
	chmod +w "$BUNDLE_A/rootfs/usr/bin/." && rm "$BUNDLE_A/rootfs/usr/bin/env"
	ln -s "a \\really //weird _00..:=path " "$BUNDLE_A/rootfs/usr/bin/env"

	# Repack the image under the same tag.
	umoci repack --image "${IMAGE}:${TAG}" "$BUNDLE_A"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Unpack it again.
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE_B"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE_B"

	# Ensure that gomtree suceeds on the old bundle, which is what this was
	# generated from.
	gomtree -p "$BUNDLE_A/rootfs" -f "$BUNDLE_B"/sha256_*.mtree
	[ "$status" -eq 0 ]
	[ -z "$output" ]

	# Just for sanity, check that everything looks okay.
	[ -f "$BUNDLE_A/rootfs/etc" ]
	[ -d "$BUNDLE_A/rootfs/bin/sh" ]
	[ -L "$BUNDLE_A/rootfs/usr/bin/env" ]

	# Make sure that the new layer is a non-empty_layer.
	umoci stat --image "${IMAGE}:${TAG}" --json
	[ "$status" -eq 0 ]
	[[ "$(echo "$output" | jq -SM '.history[-1].empty_layer')" == "null" ]]

	image-verify "${IMAGE}"
}

@test "umoci repack --history.*" {
	BUNDLE="$(setup_tmpdir)"

	image-verify "${IMAGE}"

	# Unpack the image.
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	# Make some small change.
	touch "$BUNDLE/a_small_change"
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

@test "umoci {un,re}pack [hardlink]" {
	BUNDLE_A="$(setup_tmpdir)"
	BUNDLE_B="$(setup_tmpdir)"

	image-verify "${IMAGE}"

	# Unpack the image.
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE_A"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE_A"

	# Create a file and some hardlinks.
	echo "this has some contents" >> "$BUNDLE_A/rootfs/small_change"
	ln -f "$BUNDLE_A/rootfs/small_change" "$BUNDLE_A/rootfs/link_hard"
	mkdir -p "$BUNDLE_A/rootfs/tmp" && ln -f "$BUNDLE_A/rootfs/small_change" "$BUNDLE_A/rootfs/tmp/link_hard"
	mkdir -p "$BUNDLE_A/rootfs/another/link/dir" && ln -f "$BUNDLE_A/rootfs/link_hard" "$BUNDLE_A/rootfs/another/link/dir/hard"

	# Symlink + hardlink.
	ln -sf "/../../.././small_change" "$BUNDLE_A/rootfs/symlink"
	ln -Pf "$BUNDLE_A/rootfs/symlink" "$BUNDLE_A/rootfs/tmp/symlink_hard"

	# Repack the image, setting history values.
	umoci repack --image "${IMAGE}:${TAG}-new" "$BUNDLE_A"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Unpack the image again.
	umoci unpack --image "${IMAGE}:${TAG}-new" "$BUNDLE_B"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE_B"

	# Now make sure that the paths all have the same inode numbers.
	sane_run stat -c 'ino=%i nlink=%h type=%f' "$BUNDLE_B/rootfs/small_change"
	[ "$status" -eq 0 ]
	originalA="$output"
	sane_run stat -c 'ino=%i nlink=%h type=%f' "$BUNDLE_B/rootfs/link_hard"
	[ "$status" -eq 0 ]
	[[ "$output" == "$originalA" ]]
	sane_run stat -c 'ino=%i nlink=%h type=%f' "$BUNDLE_B/rootfs/tmp/link_hard"
	[ "$status" -eq 0 ]
	[[ "$output" == "$originalA" ]]
	sane_run stat -c 'ino=%i nlink=%h type=%f' "$BUNDLE_B/rootfs/another/link/dir/hard"
	[ "$status" -eq 0 ]
	[[ "$output" == "$originalA" ]]

	# Now make sure that the paths all have the same inode numbers.
	sane_run stat -c 'ino=%i nlink=%h type=%f' "$BUNDLE_B/rootfs/symlink"
	[ "$status" -eq 0 ]
	originalB="$output"
	sane_run stat -c 'ino=%i nlink=%h type=%f' "$BUNDLE_B/rootfs/tmp/symlink_hard"
	[ "$status" -eq 0 ]
	[[ "$output" == "$originalB" ]]

	# Make sure that hardlink->symlink != hardlink.
	[[ "$originalA" != "$originalB" ]]

	image-verify "${IMAGE}"
}

@test "umoci {un,re}pack [unpriv]" {
	BUNDLE_A="$(setup_tmpdir)"
	BUNDLE_B="$(setup_tmpdir)"

	image-verify "${IMAGE}"

	# Unpack the image.
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE_A"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE_A"

	# Create some directories for unpriv check.
	mkdir -p "$BUNDLE_A/rootfs/some/directory/path"

	# mkfifo and some other stuff
	mkfifo "$BUNDLE_A/rootfs/some/directory/path/fifo"
	echo "some contents" >> "$BUNDLE_A/rootfs/some/directory/path/file"
	mkdir "$BUNDLE_A/rootfs/some/directory/path/dir"
	ln -s "/../././././/../../../../etc/shadow" "$BUNDLE_A/rootfs/some/directory/path/link"

	# Chmod.
	chmod 0000 "$BUNDLE_A/rootfs/some/directory/path"
	chmod 0000 "$BUNDLE_A/rootfs/some/directory"
	chmod 0000 "$BUNDLE_A/rootfs/some"

	# Repack the image.
	umoci repack --image "${IMAGE}" "$BUNDLE_A"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Unpack the image again.
	umoci unpack --image "${IMAGE}" "$BUNDLE_B"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE_B"

	# Undo the chmodding.
	chmod +rwx "$BUNDLE_B/rootfs/some"
	chmod +rwx "$BUNDLE_B/rootfs/some/directory"
	chmod +rwx "$BUNDLE_B/rootfs/some/directory/path"

	# Make sure the types are right.
	[[ "$(stat -c '%F' "$BUNDLE_B/rootfs/some/directory/path/fifo")" == "fifo" ]]
	[[ "$(stat -c '%F' "$BUNDLE_B/rootfs/some/directory/path/file")" == "regular file" ]]
	[[ "$(stat -c '%F' "$BUNDLE_B/rootfs/some/directory/path/dir")" == "directory" ]]
	[[ "$(stat -c '%F' "$BUNDLE_B/rootfs/some/directory/path/link")" == "symbolic link" ]]

	image-verify "${IMAGE}"
}

@test "umoci {un,re}pack [xattrs]" {
	BUNDLE_A="$(setup_tmpdir)"
	BUNDLE_B="$(setup_tmpdir)"
	BUNDLE_C="$(setup_tmpdir)"

	image-verify "${IMAGE}"

	# Unpack the image.
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE_A"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE_A"

	# Make a test directory to make sure nesting works.
	mkdir -p "$BUNDLE_A/rootfs/some/test/directory"
	xattr -w user.toplevel.some "some directory" "$BUNDLE_A/rootfs/some"
	xattr -w user.midlevel.test "test directory" "$BUNDLE_A/rootfs/some/test"
	xattr -w user.lowlevel.direct "directory" "$BUNDLE_A/rootfs/some/test/directory"

	# Set user.* xattrs.
	chmod +w "$BUNDLE_A/rootfs/root" && xattr -w user.some.value thisisacoolfile    "$BUNDLE_A/rootfs/root"
	chmod +w "$BUNDLE_A/rootfs/etc"  && xattr -w user.another    valuegoeshere      "$BUNDLE_A/rootfs/etc"
	chmod +w "$BUNDLE_A/rootfs/var"  && xattr -w user.3rd        halflife3confirmed "$BUNDLE_A/rootfs/var"
	chmod +w "$BUNDLE_A/rootfs/usr"  && xattr -w user."key also" "works if you try" "$BUNDLE_A/rootfs/usr"
	chmod +w "$BUNDLE_A/rootfs/lib"  && xattr -w user.empty_cont ""                 "$BUNDLE_A/rootfs/lib"

	# Repack the image.
	umoci repack --image "${IMAGE}" "$BUNDLE_A"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Unpack the image again.
	umoci unpack --image "${IMAGE}" "$BUNDLE_B"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE_B"

	# Make sure the xattrs have been set.
	sane_run xattr -p user.toplevel.some "$BUNDLE_B/rootfs/some"
	[ "$status" -eq 0 ]
	[[ "$output" == "some directory" ]]
	sane_run xattr -p user.midlevel.test "$BUNDLE_B/rootfs/some/test"
	[ "$status" -eq 0 ]
	[[ "$output" == "test directory" ]]
	sane_run xattr -p user.lowlevel.direct "$BUNDLE_B/rootfs/some/test/directory"
	[ "$status" -eq 0 ]
	[[ "$output" == "directory" ]]
	sane_run xattr -p user.some.value "$BUNDLE_B/rootfs/root"
	[ "$status" -eq 0 ]
	[[ "$output" == "thisisacoolfile" ]]
	sane_run xattr -p user.another "$BUNDLE_B/rootfs/etc"
	[ "$status" -eq 0 ]
	[[ "$output" == "valuegoeshere" ]]
	sane_run xattr -p user.3rd "$BUNDLE_B/rootfs/var"
	[ "$status" -eq 0 ]
	[[ "$output" == "halflife3confirmed" ]]
	sane_run xattr -p user."key also" "$BUNDLE_B/rootfs/usr"
	[ "$status" -eq 0 ]
	[[ "$output" == "works if you try" ]]
	# Empty-valued xattrs are disallowed by PAX.
	sane_run xattr -p user.empty_cont "$BUNDLE_B/rootfs/lib"
	[[ "$output" == *"No such xattr: user.empty_cont"* ]]

	# Now make some changes.
	xattr -d user.some.value "$BUNDLE_B/rootfs/root"
	xattr -d user.midlevel.test "$BUNDLE_B/rootfs/some/test"
	xattr -w user.3rd "jk, hl3 isn't here yet" "$BUNDLE_B/rootfs/var"

	# Repack the image.
	umoci repack --image "${IMAGE}" "$BUNDLE_B"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Unpack the image again.
	umoci unpack --image "${IMAGE}" "$BUNDLE_C"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE_C"

	# Make sure the xattrs have been set.
	sane_run xattr -p user.toplevel.some "$BUNDLE_C/rootfs/some"
	[ "$status" -eq 0 ]
	[[ "$output" == "some directory" ]]
	sane_run xattr -p user.midlevel.test "$BUNDLE_C/rootfs/some/test"
	[[ "$output" == *"No such xattr: user.midlevel.test"* ]]
	sane_run xattr -p user.lowlevel.direct "$BUNDLE_C/rootfs/some/test/directory"
	[ "$status" -eq 0 ]
	[[ "$output" == "directory" ]]
	sane_run xattr -p user.some.value "$BUNDLE_C/rootfs/root"
	[[ "$output" == *"No such xattr: user.some.value"* ]]
	sane_run xattr -p user.another "$BUNDLE_C/rootfs/etc"
	[ "$status" -eq 0 ]
	[[ "$output" == "valuegoeshere" ]]
	sane_run xattr -p user.3rd "$BUNDLE_C/rootfs/var"
	[ "$status" -eq 0 ]
	[[ "$output" == "jk, hl3 isn't here yet" ]]
	sane_run xattr -p user."key also" "$BUNDLE_C/rootfs/usr"
	[ "$status" -eq 0 ]
	[[ "$output" == "works if you try" ]]
	# Empty-valued xattrs are disallowed by PAX.
	sane_run xattr -p user.empty_cont "$BUNDLE_C/rootfs/lib"
	[[ "$output" == *"No such xattr: user.empty_cont"* ]]

	image-verify "${IMAGE}"
}

@test "umoci {un,re}pack [unicode]" {
	BUNDLE_A="$(setup_tmpdir)"
	BUNDLE_B="$(setup_tmpdir)"
	BUNDLE_C="$(setup_tmpdir)"

	image-verify "${IMAGE}"

	# Unpack the image.
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE_A"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE_A"

	# Unicode is very fun.
	mkdir "$BUNDLE_A/rootfs/TÜBİTAK_UEKAE_Kök_Sertifika_ Hizmet Sağlayıcısı -_Sürüm_3"
	touch "$BUNDLE_A/rootfs/TÜBİTAK_UEKAE_Kök_Sertifika_ Hizmet Sağlayıcısı -_Sürüm_3/NetLock_Arany_=Class_Gold=_Főtanúsítvány.pem"
	touch "$BUNDLE_A/rootfs/AC_Raíz_Certicámara_S.A..pem"
	touch "$BUNDLE_A/rootfs/ <-- some more weird characters --> 你好，世界"

	# Repack the image.
	umoci repack --image "${IMAGE}" "$BUNDLE_A"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Unpack the image again.
	umoci unpack --image "${IMAGE}" "$BUNDLE_B"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE_B"

	# Make sure the directories and files exist.
	[ -d "$BUNDLE_B/rootfs/TÜBİTAK_UEKAE_Kök_Sertifika_ Hizmet Sağlayıcısı -_Sürüm_3" ]
	[ -f "$BUNDLE_B/rootfs/TÜBİTAK_UEKAE_Kök_Sertifika_ Hizmet Sağlayıcısı -_Sürüm_3/NetLock_Arany_=Class_Gold=_Főtanúsítvány.pem" ]
	[ -f "$BUNDLE_B/rootfs/AC_Raíz_Certicámara_S.A..pem" ]
	[ -f "$BUNDLE_B/rootfs/ <-- some more weird characters --> 你好，世界" ]

	# Now make some changes.
	rm "$BUNDLE_B/rootfs/AC_Raíz_Certicámara_S.A..pem"

	# Repack the image.
	umoci repack --image "${IMAGE}" "$BUNDLE_B"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Unpack the image again.
	umoci unpack --image "${IMAGE}" "$BUNDLE_C"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE_C"

	# Make sure the directories and files exist.
	[ -d "$BUNDLE_C/rootfs/TÜBİTAK_UEKAE_Kök_Sertifika_ Hizmet Sağlayıcısı -_Sürüm_3" ]
	[ -f "$BUNDLE_C/rootfs/TÜBİTAK_UEKAE_Kök_Sertifika_ Hizmet Sağlayıcısı -_Sürüm_3/NetLock_Arany_=Class_Gold=_Főtanúsítvány.pem" ]
	! [ -f "$BUNDLE_C/rootfs/AC_Raíz_Certicámara_S.A..pem" ]
	[ -f "$BUNDLE_C/rootfs/ <-- some more weird characters --> 你好，世界" ]

	image-verify "${IMAGE}"
}

@test "umoci repack [--config.volumes]" {
	BUNDLE_A="$(setup_tmpdir)"
	BUNDLE_B="$(setup_tmpdir)"
	BUNDLE_C="$(setup_tmpdir)"
	BUNDLE_D="$(setup_tmpdir)"

	# Set some paths to be volumes.
	umoci config --image "${IMAGE}:${TAG}" --config.volume /volume --config.volume "/some nutty/path name/ here"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Unpack the image.
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE_A"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE_A"

	# Create files in those volumes.
	mkdir -p "$BUNDLE_A/rootfs/some nutty/path name/"
	echo "this is a test" > "$BUNDLE_A/rootfs/some nutty/path name/ here"
	mkdir -p "$BUNDLE_A/rootfs/volume"
	echo "another test" > "$BUNDLE_A/rootfs/volume/test"
	# ... and some outside.
	echo "more tests" > "$BUNDLE_A/rootfs/some nutty/path "
	mkdir -p "$BUNDLE_A/rootfs/some/volume"
	echo "in a mirror directory" > "$BUNDLE_A/rootfs/some/volume/test"
	echo "checking mirror" > "$BUNDLE_A/rootfs/volumetest"

	# Repack the image under a new tag.
	umoci repack --image "${IMAGE}:${TAG}-new" "$BUNDLE_A"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Re-extract to verify the volume changes weren't included.
	umoci unpack --image "${IMAGE}:${TAG}-new" "$BUNDLE_B"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE_B"

	# Check the files.
	[ -f "$BUNDLE_B/rootfs/some nutty/path " ]
	[[ "$(cat "$BUNDLE_B/rootfs/some nutty/path ")" == "more tests" ]]
	[ -d "$BUNDLE_B/rootfs/some/volume" ]
	[ -f "$BUNDLE_B/rootfs/some/volume/test" ]
	[[ "$(cat "$BUNDLE_B/rootfs/some/volume/test")" == "in a mirror directory" ]]
	[ -f "$BUNDLE_B/rootfs/volumetest" ]
	[[ "$(cat "$BUNDLE_B/rootfs/volumetest")" == "checking mirror" ]]

	# Volume paths must not be included.
	! [ -e "$BUNDLE_B/rootfs/volume" ]
	! [ -e "$BUNDLE_B/rootfs/volume/test" ]
	! [ -e "$BUNDLE_B/rootfs/some nutty/path name" ]
	! [ -e "$BUNDLE_B/rootfs/some nutty/path name/ here" ]

	# Repack a copy with volumes not masked.
	umoci repack --image "${IMAGE}:${TAG}-nomask" --no-mask-volumes "$BUNDLE_A"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Extract the no-mask variant to make sure that masked changes *were* included.
	umoci unpack --image "${IMAGE}:${TAG}-nomask" "$BUNDLE_C"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE_C"

	# Check the files.
	[ -f "$BUNDLE_C/rootfs/some nutty/path " ]
	[[ "$(cat "$BUNDLE_C/rootfs/some nutty/path ")" == "more tests" ]]
	[ -d "$BUNDLE_C/rootfs/some/volume" ]
	[ -f "$BUNDLE_C/rootfs/some/volume/test" ]
	[[ "$(cat "$BUNDLE_C/rootfs/some/volume/test")" == "in a mirror directory" ]]
	[ -f "$BUNDLE_C/rootfs/volumetest" ]
	[[ "$(cat "$BUNDLE_C/rootfs/volumetest")" == "checking mirror" ]]

	# Volume paths must be included, as well as their contents.
	[ -e "$BUNDLE_C/rootfs/volume" ]
	[ -f "$BUNDLE_C/rootfs/volume/test" ]
	[[ "$(cat "$BUNDLE_C/rootfs/volume/test")" == "another test" ]]
	[ -d "$BUNDLE_C/rootfs/some nutty/path name" ]
	[ -f "$BUNDLE_C/rootfs/some nutty/path name/ here" ]
	[[ "$(cat "$BUNDLE_C/rootfs/some nutty/path name/ here")" == "this is a test" ]]

	# Re-do everything but this time with --mask-path.
	umoci repack --image "${IMAGE}:${TAG}-new" --mask-path /volume "$BUNDLE_A"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Re-extract to verify the masked path changes weren't included.
	umoci unpack --image "${IMAGE}:${TAG}-new" "$BUNDLE_D"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE_D"

	# Check the files.
	[ -f "$BUNDLE_D/rootfs/some nutty/path " ]
	[[ "$(cat "$BUNDLE_D/rootfs/some nutty/path ")" == "more tests" ]]
	[ -d "$BUNDLE_D/rootfs/some/volume" ]
	[ -f "$BUNDLE_D/rootfs/some/volume/test" ]
	[[ "$(cat "$BUNDLE_D/rootfs/some/volume/test")" == "in a mirror directory" ]]
	[ -f "$BUNDLE_D/rootfs/volumetest" ]
	[[ "$(cat "$BUNDLE_D/rootfs/volumetest")" == "checking mirror" ]]

	# Masked paths must not be included.
	! [ -e "$BUNDLE_D/rootfs/volume" ]
	! [ -e "$BUNDLE_D/rootfs/volume/test" ]
	# And volumes will also not be included.
	! [ -e "$BUNDLE_D/rootfs/some nutty/path name" ]
	! [ -e "$BUNDLE_D/rootfs/some nutty/path name/ here" ]
}

@test "umoci repack [--refresh-bundle]" {
	BUNDLE_A="$(setup_tmpdir)"
	BUNDLE_B="$(setup_tmpdir)"
	BUNDLE_C="$(setup_tmpdir)"

	# Unpack the original image
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE_A"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE_A"

	# Make sure the files we're creating don't exist.
	! [ -e "$BUNDLE_A/rootfs/newfile" ]
	! [ -e "$BUNDLE_A/rootfs/newdir" ]
	! [ -e "$BUNDLE_A/rootfs/newdir/anotherfile" ]

	# Create them.
	echo "first file" > "$BUNDLE_A/rootfs/newfile"
	mkdir "$BUNDLE_A/rootfs/newdir"
	echo "subfile" > "$BUNDLE_A/rootfs/newdir/anotherfile"

	# Repack the image under a new tag, refreshing the bundle metadata.
	umoci repack --refresh-bundle --image "${IMAGE}:${TAG}-new" "$BUNDLE_A"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Ensure the gomtree has been refreshed in the bundle
	gomtree -p "$BUNDLE_A/rootfs" -f "$BUNDLE_A"/sha256_*.mtree
	[ "$status" -eq 0 ]
	[ -z "$output" ]

	# Unpack it again.
	umoci unpack --image "${IMAGE}:${TAG}-new" "$BUNDLE_B"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE_B"

	# Ensure that gomtree succeeds across bundles - they should be the same rootfs
	# and have the same mtree manifest
	gomtree -p "$BUNDLE_A/rootfs" -f "$BUNDLE_B"/sha256_*.mtree
	[ "$status" -eq 0 ]
	[ -z "$output" ]
	gomtree -p "$BUNDLE_B/rootfs" -f "$BUNDLE_A"/sha256_*.mtree
	[ "$status" -eq 0 ]
	[ -z "$output" ]

	# Make some other changes to the first bundle
	echo "second file" > "$BUNDLE_A/rootfs/newfile2"

	# Repack under a new tag again, without refreshing the metadata.
	umoci repack --image "${IMAGE}:${TAG}-new2" "$BUNDLE_A"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Unpack it again into a new bundle.
	umoci unpack --image "${IMAGE}:${TAG}-new2" "$BUNDLE_C"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE_C"

	# Ensure all changes are reflected
	gomtree -p "$BUNDLE_A/rootfs" -f "$BUNDLE_C"/sha256_*.mtree
	[ "$status" -eq 0 ]
	[ -z "$output" ]
	gomtree -p "$BUNDLE_C/rootfs" -f "$BUNDLE_C"/sha256_*.mtree
	[ "$status" -eq 0 ]
	[ -z "$output" ]

	# Final bundle sanity check
	[ -f "$BUNDLE_C/rootfs/newfile" ]
	[ -d "$BUNDLE_C/rootfs/newdir" ]
	[ -f "$BUNDLE_C/rootfs/newdir/anotherfile" ]
	[ -f "$BUNDLE_C/rootfs/newfile2" ]

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
