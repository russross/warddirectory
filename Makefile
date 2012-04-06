all: warddirectory-windows-32bit.zip \
	warddirectory-windows-64bit.zip \
	warddirectory-linux-32bit.tar.gz \
	warddirectory-linux-64bit.tar.gz

warddirectory-windows-32bit.zip: *.go data/*.go
	rm -f warddirectory-windows-32bit.zip
	CGO_ENABLED=0 GOOS=windows GOARCH=386 go build
	zip warddirectory-windows-32bit.zip warddirectory.exe
	rm -f warddirectory.exe

warddirectory-windows-64bit.zip: *.go data/*.go
	rm -f warddirectory-windows-64bit.zip
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build
	zip warddirectory-windows-64bit.zip warddirectory.exe
	rm -f warddirectory.exe

warddirectory-linux-32bit.tar.gz: *.go data/*.go
	rm -f warddirectory-linux-32bit.zip
	CGO_ENABLED=0 GOOS=linux GOARCH=386 go build
	tar zcvf warddirectory-linux-32bit.tar.gz warddirectory
	rm -f warddirectory

warddirectory-linux-64bit.tar.gz: *.go data/*.go
	rm -f warddirectory-linux-64bit.zip
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build
	tar zcvf warddirectory-linux-64bit.tar.gz warddirectory

clean:
	rm -f *.zip *.tar.gz
	go clean
