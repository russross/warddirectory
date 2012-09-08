all: *.go base64zipdata.go
	go build

dist: warddirectory-windows-32bit.zip \
	warddirectory-windows-64bit.zip \
	warddirectory-linux-32bit.tar.gz \
	warddirectory-linux-64bit.tar.gz

warddirectory-windows-32bit.zip: *.go base64zipdata.go
	rm -f warddirectory-windows-32bit.zip
	CGO_ENABLED=0 GOOS=windows GOARCH=386 go build
	zip warddirectory-windows-32bit.zip warddirectory.exe
	rm -f warddirectory.exe

warddirectory-windows-64bit.zip: *.go base64zipdata.go
	rm -f warddirectory-windows-64bit.zip
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build
	zip warddirectory-windows-64bit.zip warddirectory.exe
	rm -f warddirectory.exe

warddirectory-linux-32bit.tar.gz: *.go base64zipdata.go
	rm -f warddirectory-linux-32bit.zip
	CGO_ENABLED=0 GOOS=linux GOARCH=386 go build
	tar zcvf warddirectory-linux-32bit.tar.gz warddirectory
	rm -f warddirectory

warddirectory-linux-64bit.tar.gz: *.go base64zipdata.go
	rm -f warddirectory-linux-64bit.zip
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build
	tar zcvf warddirectory-linux-64bit.tar.gz warddirectory

base64zipdata.go: data/*
	echo 'package main' > base64zipdata.go
	echo -n 'var base64ZipData = `' >> base64zipdata.go
	zip -9 - data/* | base64 >> base64zipdata.go
	echo '`' >> base64zipdata.go

clean:
	rm -f *.zip *.tar.gz base64zipdata.go
	go clean
