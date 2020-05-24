
TINYGO=tinygo

smoketest:
	# Test all examples (and some boards)
	$(TINYGO) build -o test.hex -size=short -target=pca10040-s132v6       ./examples/advertisement
	@md5sum test.hex
	$(TINYGO) build -o test.hex -size=short -target=pca10040-s132v6       ./examples/heartrate
	@md5sum test.hex
	$(TINYGO) build -o test.hex -size=short -target=reelboard-s140v7      ./examples/ledcolor
	@md5sum test.hex
	# Test some more boards that are not tested above.
	$(TINYGO) build -o test.hex -size=short -target=pca10056-s140v7       ./examples/advertisement
	@md5sum test.hex
	# Test on the host
	go build -o /tmp/go-build-discard ./examples/advertisement
	go build -o /tmp/go-build-discard ./examples/heartrate
