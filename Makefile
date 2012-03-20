all:	directory.pdf

# get the membership data from my downloads directory
# change this to match your unit number and/or the location
# where you store the downloaded directory info. Then uncomment
# the lines below so that "make" will copy it over automatically.

#membership.csv:	~/Downloads/123456.csv
#	cp ~/Downloads/123456.csv ./membership.csv

directory.pdf:	membership.csv config.json *.go
	go run *.go membership.csv

clean:
	go clean
	rm -f directory.pdf membership.csv
