Ward Directory Generator
========================

This program generates a printable list of families for an LDS ward.
It uses the CSV data downloaded from an lds.org account and
processes it into a PDF file that fits everything on two pages (a
single double-sided sheet of paper).


Downloading
-----------

With Go 1 and git installed:

    go get github.com/russross/warddirectory

will download the source, compile it, and install it as a tool
called `warddirectory` in your `$GOPATH/bin` directory. Note that
you should have `$GOPATH` set up before running this command.


Running the tool
----------------

To generate a directory, go into the
`$GOPATH/src/github.com/russross/warddirectory` directory, which is
where the code was downloaded.

Download your latest ward directory information from lds.org.
Follow this link:

* <https://www.lds.org/directory?lang=eng>

You will be required to log in, then you should see a directory
listing for your ward. On the list of links at the left side, find
"Export Households" and click on it. Click on the "Download..."
button on the box that pops up.

Take the file that you downloaded and move it into the same
directory, renaming it to "membership.csv".

Within that directory, run:

    make

This should generate a file called `directory.pdf` with your ward
directory, ready for printing.


Customizing the output
----------------------

In the `warddirectory` directory, edit the file `config.json` to
customize the output. This file has a specific format that you must
take care to preserve. You can edit the following:

*   Title: the title that will be displayed in the center of the
    directory header.

*   Disclaimer: the disclaimer string that will be displayed on the
    right side of the header.

*   DateFormat: the format the date will be displayed in on the left
    side of the header. If you modify this, just the date January 2,
    2006, formatted how you would like it.

*   PageWidth, PageHeight, TopMargin, BottomMargin, LeftMargin,
    RightMargin: the size of the page and margin sizes. These are
    all in *points*, where a point is 1/72 of an inch. Note that the
    header is displayed in the top margin.

*   ColumnsPerPage: the number of columns to display on each page.

*   ColumnSep: the size of the gap between columns, also in points.

*   LeadingMultiplier: the spacing between lines of text. The total
    size of a line will be the point size of the font multiplied by
    this number.

*   MinimumFontSize, MaximumFontSize: the font size is automatically
    computed to fit the listing exactly on two pages. This gives the
    range of font sizes that should be searched.

*   MinimumSpaceMultiplier: sometimes spaces are squeezed to fit an
    extra word on a line. This is the minimum size a space will be
    allowed (as a fraction of the default size).

*   MinimumLineHeightMultiplier: same for line heights, which may be
    squeezed to fit an extra line or two into a column.

*   FontSizePrecision: when searching for the largest font that will
    allow everything to fit on two pages, it will search font sizes
    until it has compared the results with two font sizes this far
    apart (in points).

*   TitleFontMultiplier: how much bigger the title should be than
    the rest of the text. The value I use is the square root of 2.

*   FirstLineDedentMultiplier: the size of the indentation of every
    line after the first for a single family. The value I use is the
    golden ratio.

*   PhoneRegexps: a list of regular expressions that will be applied
    to phone numbers, in order. The regular expression syntax
    supported is described here:

    * <http://code.google.com/p/re2/wiki/Syntax>

    I use this to normalize phone numbers to 123-456-7890 format,
    and then to remove the area code if it is the standard one for
    my part of the state.

*   AddressRegexps: same for addresses. Note that the data contains
    address info in this format:

    123 Some Street City Name, State Zip-code

    You can use regular expressions to remove/shorten the zip code,
    abbreviate/remove the state, and/or abbreviate/remove the city.
    In my config file, I remove the state and zip code. I also
    remove the city name if it matches the default, and I put a
    comma in front of it (between the address and city) for
    non-default city names.
