
TINYGO=tinygo

smoketest: smoketest-tinygo smoketest-linux smoketest-windows

smoketest-tinygo:
	# Test all examples (and some boards)
	$(TINYGO) build -o test.hex -size=short -target=pca10040-s132v6       ./examples/advertisement
	@md5sum test.hex
	$(TINYGO) build -o test.hex -size=short -target=pca10040-s132v6       ./examples/heartrate
	@md5sum test.hex
	$(TINYGO) build -o test.hex -size=short -target=reelboard-s140v7      ./examples/ledcolor
	@md5sum test.hex
	$(TINYGO) build -o test.hex -size=short -target=pca10040-s132v6       ./examples/nusserver
	@md5sum test.hex
	$(TINYGO) build -o test.hex -size=short -target=pca10040-s132v6       ./examples/scanner
	@md5sum test.hex
	# Test some more boards that are not tested above.
	$(TINYGO) build -o test.hex -size=short -target=pca10056-s140v7       ./examples/advertisement
	@md5sum test.hex
	$(TINYGO) build -o test.hex -size=short -target=microbit-s110v8       ./examples/nusserver
	@md5sum test.hex

smoketest-linux:
	# Test on Linux.
	GOOS=linux go build -o /tmp/go-build-discard ./examples/advertisement
	GOOS=linux go build -o /tmp/go-build-discard ./examples/heartrate
	GOOS=linux go build -o /tmp/go-build-discard ./examples/nusserver
	GOOS=linux go build -o /tmp/go-build-discard ./examples/scanner

smoketest-windows:
	# Test on Windows.
	GOOS=windows CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc go build -o /tmp/go-build-discard ./examples/scanner
