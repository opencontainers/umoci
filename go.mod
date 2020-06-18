module github.com/opencontainers/umoci

go 1.14

require (
	github.com/apex/log v1.4.0
	github.com/cpuguy83/go-md2man/v2 v2.0.0 // indirect
	github.com/cyphar/filepath-securejoin v0.2.2
	github.com/docker/go-units v0.4.0
	github.com/golang/protobuf v1.4.2
	github.com/klauspost/compress v1.10.9 // indirect
	github.com/klauspost/pgzip v1.2.4
	github.com/mohae/deepcopy v0.0.0-20170929034955-c48cc78d4826
	github.com/opencontainers/go-digest v1.0.0
	github.com/opencontainers/image-spec v1.0.1
	github.com/opencontainers/runtime-spec v1.0.2
	github.com/pkg/errors v0.9.1
	github.com/rootless-containers/proto v0.1.0
	github.com/sirupsen/logrus v1.6.0 // indirect
	github.com/urfave/cli v1.22.1
	github.com/vbatts/go-mtree v0.5.0
	golang.org/x/crypto v0.0.0-20200604202706-70a84ac30bf9 // indirect
	golang.org/x/net v0.0.0-20200602114024-627f9648deb9
	golang.org/x/sys v0.0.0-20200615200032-f1bc736245b1
	google.golang.org/protobuf v1.24.0 // indirect
)

exclude (
	// These versions have regressions that break umoci's CI and UX. For more
	// details, see <https://github.com/urfave/cli/issues/1152>.
	github.com/urfave/cli v1.22.2
	github.com/urfave/cli v1.22.3
	github.com/urfave/cli v1.22.4
)
