all:	directory.pdf

# get the membership data from my downloads directory
# change this to match your unit number and/or the location
# where you store the downloaded directory info
membership.csv:	~/Downloads/373621.csv
	cp ~/Downloads/373621.csv ./membership.csv

directory.pdf:	membership.csv *.go
	go run *.go < membership.csv > directory.pdf

clean:
	go clean
	rm -f directory.pdf membership.csv
