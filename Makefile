
TINYGO=tinygo

smoketest: smoketest-tinygo smoketest-linux smoketest-windows

smoketest-tinygo:
	# Test all examples (and some boards)
	$(TINYGO) build -o test.hex -size=short -target=pca10040-s132v6       ./examples/advertisement
	@md5sum test.hex
	$(TINYGO) build -o test.uf2 -size=short -target=circuitplay-bluefruit ./examples/advertisement
	@md5sum test.hex
	$(TINYGO) build -o test.uf2 -size=short -target=circuitplay-bluefruit ./examples/circuitplay
	@md5sum test.hex
	$(TINYGO) build -o test.hex -size=short -target=circuitplay-bluefruit ./examples/connparams
	@md5sum test.hex
	$(TINYGO) build -o test.uf2 -size=short -target=circuitplay-bluefruit ./examples/discover
	@md5sum test.hex
	$(TINYGO) build -o test.hex -size=short -target=pca10040-s132v6       ./examples/heartrate
	@md5sum test.hex
	$(TINYGO) build -o test.hex -size=short -target=reelboard-s140v7      ./examples/ledcolor
	@md5sum test.hex
	$(TINYGO) build -o test.hex -size=short -target=pca10040-s132v6       ./examples/nusclient
	@md5sum test.hex
	$(TINYGO) build -o test.hex -size=short -target=pca10040-s132v6       ./examples/nusserver
	@md5sum test.hex
	$(TINYGO) build -o test.hex -size=short -target=pca10040-s132v6       ./examples/scanner
	@md5sum test.hex
	$(TINYGO) build -o test.hex -size=short -target=pca10040-s132v6       ./examples/stop-advertisement
	@md5sum test.hex
	# Test some more boards that are not tested above.
	$(TINYGO) build -o test.hex -size=short -target=pca10056-s140v7       ./examples/advertisement
	@md5sum test.hex
	$(TINYGO) build -o test.hex -size=short -target=microbit-s110v8       ./examples/nusserver
	@md5sum test.hex
	$(TINYGO) build -o test.hex -size=short -target=microbit-v2-s113v7    ./examples/nusserver
	@md5sum test.hex
	$(TINYGO) build -o test.uf2 -size=short -target=nano-rp2040 		  ./examples/discover
	@md5sum test.hex
	$(TINYGO) build -o test.uf2 -size=short -target=arduino-nano33 		  ./examples/discover
	@md5sum test.hex
	$(TINYGO) build -o test.uf2 -size=short -target=pyportal	          ./examples/discover
	@md5sum test.hex
	$(TINYGO) build -o test.uf2 -size=short -target=nano-rp2040 		  ./examples/advertisement
	@md5sum test.hex
	$(TINYGO) build -o test.uf2 -size=short -target=circuitplay-express -tags="hci hci_uart" ./examples/advertisement
	@md5sum test.hex

smoketest-linux:
	# Test on Linux.
	GOOS=linux go build -o /tmp/go-build-discard ./examples/advertisement
	GOOS=linux go build -o /tmp/go-build-discard ./examples/connparams
	GOOS=linux go build -o /tmp/go-build-discard ./examples/heartrate
	GOOS=linux go build -o /tmp/go-build-discard ./examples/heartrate-monitor
	GOOS=linux go build -o /tmp/go-build-discard ./examples/nusserver
	GOOS=linux go build -o /tmp/go-build-discard ./examples/scanner
	GOOS=linux go build -o /tmp/go-build-discard ./examples/discover

smoketest-windows:
	# Test on Windows.
	GOOS=windows go build -o /tmp/go-build-discard ./examples/scanner
	GOOS=windows go build -o /tmp/go-build-discard ./examples/discover
	GOOS=windows go build -o /tmp/go-build-discard ./examples/heartrate-monitor
	GOOS=windows go build -o /tmp/go-build-discard ./examples/advertisement
	GOOS=windows go build -o /tmp/go-build-discard ./examples/heartrate

smoketest-macos:
	# Test on macos.
	GOOS=darwin CGO_ENABLED=1 go build -o /tmp/go-build-discard ./examples/scanner
	GOOS=darwin CGO_ENABLED=1 go build -o /tmp/go-build-discard ./examples/discover
	GOOS=darwin CGO_ENABLED=1 go build -o /tmp/go-build-discard ./examples/nusclient
	GOOS=darwin CGO_ENABLED=1 go build -o /tmp/go-build-discard ./examples/heartrate-monitor

gen-uuids:
	# generate the standard service and characteristic UUIDs
	go run ./tools/gen-service-uuids/main.go
	go run ./tools/gen-characteristic-uuids/main.go
