# Fuzzing umoci

Umoci has a series of fuzz tests. These are implemented by way of [go-fuzz](https://github.com/dvyukov/go-fuzz).

## Running the fuzzers

To run the fuzzers, first build the fuzzer image from the root of this repository:

```bash
sudo docker build -t umoci-fuzz -f Dockerfile.fuzz .
```
Next, get a shell in the container:
```bash
sudo docker run -it umoci-fuzz
```
At this point, you can navigate to any directory that has a fuzzer and build it:

```bash
cd $PATH_TO_FUZZER
go-fuzz-build -libfuzzer -func=FUZZ_NAME && \
clang -fsanitize=fuzzer PACKAGE_NAME.a -o fuzzer
```
`FUZZ_NAME` will typically be `Fuzz`, but in some cases the respective fuzzers will have more descriptive names. 

If you encounter any errors when linking with `PACKAGE_NAME.a`, simply `ls` after running `go-fuzz-build...`, and you will see the archive to link with.

If everything goes well until this point, you can run the fuzzer:
```bash
./fuzzer
```
