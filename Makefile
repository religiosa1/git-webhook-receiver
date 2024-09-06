name=git-webhook-receiver

all: release-unix release-windows

release-unix:
	mkdir -p build; \
	for arch in 386 arm amd64 arm64 ; do \
		echo "Building linux-$$arch"; \
		GOOS=linux GOARCH=$$arch go build -o build/${name}-linux-$$arch/${name}; \
		tar cz -C build -f build/${name}-linux-$$arch.tar.gz ${name}-linux-$$arch; \
	done

release-windows:
	mkdir -p build; \
	for arch in 386 amd64 arm; do \
		echo "Building windows-$$arch"; \
		GOOS=windows GOARCH=$$arch go build -o build/${name}-win-$$arch/${name}.exe; \
		tar cz -C build -f build/${name}-win-$$arch.tar.gz ${name}-win-$$arch; \
	done

clean:
	rm -rf build