# Reggie

[![GitHub Actions status](https://github.com/bloodorangeio/reggie/workflows/build/badge.svg)](https://github.com/bloodorangeio/reggie/actions?query=workflow%3Abuild) [![GoDoc](https://godoc.org/github.com/bloodorangeio/reggie?status.svg)](https://godoc.org/github.com/bloodorangeio/reggie)

![](https://raw.githubusercontent.com/bloodorangeio/reggie/master/reggie.png)

Reggie is a dead simple Go HTTP client designed to be used against [OCI Distribution](https://github.com/opencontainers/distribution-spec), built on top of [Resty](https://github.com/go-resty/resty).

There is also built-in support for both basic auth and "Docker-style" token auth.

*Note: Authentication/authorization is not part of the distribution spec, but it has been implemented similarly across registry providers targeting the Docker client.*

## Getting Started

First import the library:
```go
import "github.com/bloodorangeio/reggie"
```

Then construct a client:

```go
client, err := reggie.NewClient("http://localhost:5000")
```

You may also construct the client with a number of options related to authentication, etc:

```go
client, err := reggie.NewClient("https://r.mysite.io",
    reggie.WithUsernamePassword("myuser", "mypass"),  // registry credentials
    reggie.WIthDefaultName("myorg/myrepo"),           // default repo name
    reggie.WithDebug(true))                           // enable debug logging
```

## Making Requests

Reggie uses a domain-specific language to supply various parts of the URI path in order to provide visual parity with [the spec](https://github.com/opencontainers/distribution-spec/blob/master/spec.md).

For example, to list all tags for the repo `megacorp/superapp`, you might do the following:

```go
req := client.NewRequest(reggie.GET, "/v2/<name>/tags/list",
    reggie.WithName("megacorp/superapp"))
```

This will result in a request object built for `GET /v2/megacorp/superapp/tags/list`.

Finally, execute the request, which will return a response object:
```go
resp, err := client.Do(req)
fmt.Println("Status Code:", resp.StatusCode())
```

## Path Substitutions

Below is a table of all of the possible URI parameter substitutions and associated methods:


| URI Parameter | Description | Option method |
|-|-|-|
| `<name>` | Namespace of a repository within a registry | `WithDefaultName` (`Client`) or<br>`WithName` (`Request`) |
| `<digest>` | Content-addressable identifier | `WithDigest` (`Request`) |
| `<reference>` | Tag or digest | `WithReference` (`Request`) |
| `<session_id>` | Session ID for upload | `WithSessionID` (`Request`) |

## Auth

All requests are first attempted without any authentication. If an endpoint returns a `401 Unauthorized`, and the client has been constructed with a username and password (via `reggie.WithUsernamePassword`), the request is retried with an `Authorization` header.

Included in the 401 response, registries should return a `Www-Authenticate` header describing how to to authenticate.

For more info about the `Www-Authenticate` header and general HTTP auth topics, please see IETF RFCs [7235](https://tools.ietf.org/html/rfc7235) and [6749](https://tools.ietf.org/html/rfc6749).

### Basic Auth

 If the `Www-Authenticate` header contains the string "Basic", then the header used in the retried request will be formatted as `Authorization: Basic <credentials>`, where credentials is the base64 encoding of the username and password joined by a single colon.

### "Docker-style" Token Auth
*Note: most commercial registries use this method.*

If the`Www-Authenticate` contains the string "Bearer", an attempt is made to retrieve a token from an authorization service endpoint, the URL of which should be provided in the `Realm` field of the header. The header then used in the retried request will be formatted as `Authorization: Bearer <token>`, where token is the one returned from the token endpoint.

Here is a visual of this auth flow copied from the [Docker docs](https://docs.docker.com/registry/spec/auth/token/):

![](./v2-registry-auth.png)

#### Custom Auth Scope

 It may be necessary to override the `scope` obtained from the `Www-Authenticate` header in the registry's response. This can be done on the client level:

 ```
client, err := reggie.NewClient("http://localhost:5000",
    reggie.WithAuthScope("repository:mystuff/myrepo:pull,push"))
 ```

## Other Features

### Method Chaining

Each of the types provided by this package (`Client`, `Request`, & `Response`) are all built on top of types provided by Resty. In most cases, methods provided by Resty should just work on these objects (see the [godoc](https://godoc.org/github.com/go-resty/resty) for more info).

The following commonly-used methods have been wrapped in order to allow for method chaining:

- `req.Header`
- `req.SetQueryParam`
- `req.SetBody`

The following is an example of using method chaining to build a request:
```go
req := client.NewRequest(reggie.PUT, lastResponse.GetRelativeLocation()).
    SetHeader("Content-Length", configContentLength).
    SetHeader("Content-Type", "application/octet-stream").
    SetQueryParam("digest", configDigest).
    SetBody(configContent)
```

### Location Header Parsing

For certain types of requests, such as chunked uploads, the `Location` header is needed in order to make follow-up requests.

Reggie provides two helper methods to obtain the redirect location:
```go
fmt.Println("Relative location:", resp.GetRelativeLocation())  // /v2/...
fmt.Println("Absolute location:", resp.GetAbsoluteLocation())  // https://...
```

### Error Parsing

On the response object, you may call the `Errors()` method which will attempt to parse the response body into a list of [OCI ErrorInfo](https://github.com/opencontainers/distribution-spec/blob/master/specs-go/v1/error.go#L36) objects:
```go
for _, e := range resp.Errors() {
    fmt.Println("Code:",    e.Code)
    fmt.Println("Message:", e.Message)
    fmt.Println("Detail:",  e.Detail)
}
```

### HTTP Method Constants

Simply-named constants are provided for the following HTTP request methods:
```go
reggie.GET     // "GET"
reggie.PUT     // "PUT"
reggie.PATCH   // "PATCH"
reggie.DELETE  // "DELETE"
reggie.POST    // "POST"
reggie.HEAD    // "HEAD"
reggie.OPTIONS // "OPTIONS"
```

### Custom User-Agent

By default, requests made by Reggie will use a default value for the `User-Agent` header in order for registry providers to identify incoming requests:
```
User-Agent: reggie/0.3.0 (https://github.com/bloodorangeio/reggie)
```

If you wish to use a custom value for `User-Agent`, such as "my-agent" for example, you can do the following:
```go
client, err := reggie.NewClient("http://localhost:5000",
    reggie.WithUserAgent("my-agent"))
```

## Example

The following is an example of a resumable blob upload and subsequent manifest upload:

```go
package main

import (
	"fmt"

	"github.com/bloodorangeio/reggie"
	godigest "github.com/opencontainers/go-digest"
)

func main() {
	// construct client pointing to your registry
	client, err := reggie.NewClient("http://localhost:5000",
		reggie.WithDefaultName("myorg/myrepo"),
		reggie.WithDebug(true))
	if err != nil {
		panic(err)
	}

	// get the session URL
	req := client.NewRequest(reggie.POST, "/v2/<name>/blobs/uploads/")
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}

	// a blob for an empty manifest config, separated into 2 chunks ("{" and "}")
	blob := []byte("{}")
	blobChunk1 := blob[:1]
	blobChunk1Range := fmt.Sprintf("0-%d", len(blobChunk1)-1)
	blobChunk2 := blob[1:]
	blobChunk2Range := fmt.Sprintf("%d-%d", len(blobChunk1), len(blob)-1)
	blobDigest := godigest.FromBytes(blob).String()

	// upload the first chunk
	req = client.NewRequest(reggie.PATCH, resp.GetRelativeLocation()).
		SetHeader("Content-Type", "application/octet-stream").
		SetHeader("Content-Length", fmt.Sprintf("%d", len(blobChunk1))).
		SetHeader("Content-Range", blobChunk1Range).
		SetBody(blobChunk1)
	resp, err = client.Do(req)
	if err != nil {
		panic(err)
	}

	// upload the final chunk and close the session
	req = client.NewRequest(reggie.PUT, resp.GetRelativeLocation()).
		SetHeader("Content-Length", fmt.Sprintf("%d", len(blobChunk2))).
		SetHeader("Content-Range", blobChunk2Range).
		SetHeader("Content-Type", "application/octet-stream").
		SetQueryParam("digest", blobDigest).
		SetBody(blobChunk2)
	resp, err = client.Do(req)
	if err != nil {
		panic(err)
	}

	// validate the uploaded blob content
	req = client.NewRequest(reggie.GET, "/v2/<name>/blobs/<digest>",
		reggie.WithDigest(blobDigest))
	resp, err = client.Do(req)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Blob content:\n%s\n", resp.String())

	// upload the manifest (referencing the uploaded blob)
	ref := "mytag"
	manifest := []byte(fmt.Sprintf(
		"{ \"mediaType\": \"application/vnd.oci.image.manifest.v1+json\", \"config\":  { \"digest\": \"%s\", "+
			"\"mediaType\": \"application/vnd.oci.image.config.v1+json\","+" \"size\": %d }, \"layers\": [], "+
			"\"schemaVersion\": 2 }",
		blobDigest, len(blob)))
	req = client.NewRequest(reggie.PUT, "/v2/<name>/manifests/<reference>",
		reggie.WithReference(ref)).
		SetHeader("Content-Type", "application/vnd.oci.image.manifest.v1+json").
		SetBody(manifest)
	resp, err = client.Do(req)
	if err != nil {
		panic(err)
	}

	// validate the uploaded manifest content
	req = client.NewRequest(reggie.GET, "/v2/<name>/manifests/<reference>",
		reggie.WithReference(ref)).
		SetHeader("Accept", "application/vnd.oci.image.manifest.v1+json")
	resp, err = client.Do(req)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Manifest content:\n%s\n", resp.String())
}
```
