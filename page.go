package main

var indexTemplate = `<!DOCTYPE HTML PUBLIC "-//W3C//DTD HTML 4.01 Transitional//EN" "http://www.w3.org/TR/html4/loose.dtd">

<html>
<head>
<meta http-equiv="content-type" content="text/html; charset=UTF-8">
<title>Ward Directory Generator</title>
<style type="text/css">
  body {
    font-family: Georgia,Verdana,Sans-serif;
    font-size: 120%;
    background: darkblue;
    margin: 5px;
  }

  h1, h2 {
    font-weight: bold;
    margin-top: 1.25em;
  }
  fieldset {
    margin-top: 1em;
  }
  fieldset.section > legend {
    cursor: pointer;
  }
  legend {
    font-weight: bold;
    font-size: 150%;
  }
  .regexplist li {
    list-style-type: none;
  }
  ul.regexplist {
    padding-left: 1em;
  }
  .regexplist label {
    padding-left: 0;
  }
  .regexplist fieldset, .regexplist fieldset label {
    cursor: pointer;
  }
  .regexplist fieldset {
    padding-top: 0;
    padding-bottom: 0;
  }

  h1:first-child { margin-top: 5px; }
  p:first-child { margin-top: 5px; }

  div#header {
    margin-bottom: 4px;
  }

  div#header h1 {
    font-size: 300%;
    margin-bottom: 0;
  }

  #header, #maincolumn {
    width: 960px;
    margin: 0 auto;
    padding: 1em;
    background: white;
    border: 2px solid black;
  }

  label {
    width: 35%;
    float: left;
    padding-left: 2em;
    font-weight: bold;
  }

  input[type=text] {
    width: 60%;
  }

  hr {
    margin-top: 2em;
  }
</style>
<script type="text/javascript" src="/jquery.js"></script>
<script type="text/javascript">
jQuery(function ($) {
    // clicking on a section label hides/shows it
    $('fieldset.section > legend').click(function () {
        $(this).siblings().toggle();
    }).click();

    // let the user change the unit of measure
    $('#units').change(function() {
        var points_per = Number($('#units').val());
        $('.measurement').each(function() {
            var id = this.id.substr(5);
            var value = Number($('#' + id).val()) / points_per;
            this.value = value;
        });
    }).change();

    // when a measurement is changed by the user in #units,
    // update the hidden field (which is stored in points)
    $('.measurement').change(function() {
        var points_per = Number($('#units').val());
        var id = this.id.substr(5);
        var value = Number(this.value) * points_per;
        $('#' + id).val(value).change();
    });

    // forbid generating if not file has been selected
    $('#generatebutton').click(function() {
        // make sure they selected a file before trying to upload
        if ($('#MembershipData').val() == '') {
            alert('Please select a .csv file before clicking the “Generate” button');
            return false;
        }
    });

    // forbid importing if no file has been selected
    $('#importbutton').click(function() {
        // make sure they selected a file before trying to upload
        if ($('#DirectoryConfig').val() == '') {
            alert('Please select a .json file before clicking the “Import settings” button');
            return false;
        }
    });

    // confirm delete requests
    $('#deletebutton').click(function() {
        // confirm before deleting
        if (!confirm("Are you sure you want to clear all of your settings?")) {
            return false;
        }
    });

    // save in the background any time anything is updated
    $('.save').live('change', function () {
        // submit using ajax to avoid reseting the csv file field
        var data = $('#submitform').serialize();
        data['SubmitButton'] = 'Save';
        $.ajax({
            type: 'POST',
            url: '/submit',
            data: data,
            success: function () { console.log('Saved'); },
            error: function (err, msg, http) {
                alert('Oops! Failed to save! Did you shut down the server?');
                console.log('Failed save');
                console.log(err);
                console.log(msg);
                console.log(http);
            }
        });
        return false;
    });

    // renumber all regexp pairs in DOM order
    var renumber = function() {
        var last;
        // loop over each kind of regexp pair list
        $('.regexplist').each(function () {
            $(this).find('fieldset').each(function(i, elt) {
                var $elt = $(elt);
                $elt.find('label').each(function () {
                    this.htmlFor = this.htmlFor.replace(/\.\d+\./, '.' + i + '.');
                });
                $elt.find('input').each(function () {
                    last = this;
                    this.id = this.id.replace(/\.\d+\./, '.' + i + '.');
                    this.name = this.name.replace(/\.\d+\./, '.' + i + '.');
                });
            });
        });
        $(last).change();
    };

    // when a user drags and drops a regexp pair, renumber them
    $('.regexplist').sortable({
        stop: renumber
    });

    // when a new regexp pair is requested, clone an existing one
    $('.moreregexps').click(function () {
        // clone one of the existing pairs
        var $list = $(this).parent('p').prev('ul');
        var $elt = $list.children('li').last().clone().appendTo($list);

        // renumber the whole list to fix this one
        renumber();
        return false;
    });
});
</script>
</head>

<body>
<div id="header">
<h1>Ward Directory Generator</h1>
</div>

<div id="maincolumn">
<p>This app generates a printable list of families for an LDS ward.
It uses ward directory data that anyone with an lds.org account can
download and formats it on two pages. This lets you easily print a
nice ward directory that fits on one double-sided sheet of
paper.</p>

<p>Note: a ward directory contains sensitive personal information.
Please be respectful in the way that you use it, and be sure to
follow the church's guidelines. In particular, you may not use the
information for commercial or political purposes, nor may you
distribute it beyond the boundaries of your ward. You should use
this service under the direction of your Bishop or Stake President,
and should not distribute a directory without explicit approval from
the same. Also, the handbook requires that only stake or ward budget
funds be used to pay for directories; no advertising is
permitted.</p>

<p>To use it, you must first download the directory information for
your ward.  Follow this link:</p>

<ul><li><a href="https://lds.org/directory/" target="_blank">https://lds.org/directory/</a>
(opens in a new tab/window)</li></ul>

<p>You will need to log in using your lds.org ID. It should take you
to a screen with your ward directory listed. On the left side, find
a link that says “Export Households” and click on it. It will pop up
a box and let you download a file.  Download the file and take note
of where it is saved (usually in you “Downloads” directory).</p>

<form method="post" id="submitform" action="/submit" enctype="multipart/form-data">
<p>Next, come back here and click on this button and find the file:
<input type="file" id="MembershipData" name="MembershipData"></p>

<p>Finally, click on the “Generate” button here to download the
printable file:
<input type="submit" id="generatebutton" name="SubmitButton" value="Generate"></p>

<p>This will generate the ward directory PDF file and automatically
start downloading it. The membership data is never stored on the
server, nor is it used for any purpose other than to generate the
PDF file that you download.</p>

<p>Print this PDF file on a single sheet of paper and hand out to
your ward members. It is simple enough that you can print up new
ones every few months and hand them out at the beginning of
sacrament meeting.</p>

<p>If your printer/copier does not do double sided printing, print
one side first, then turn the paper over and print the other
side.</p>

<p>To shut down the generator service, click this button:
<input type="submit" id="shutdownbutton" name="SubmitButton" value="Shutdown"></p>

<h1>Customize the output</h1>

<p>Click on any of these section headers to see the ways that you
can customize your directory. At a minimum, open the “Page header”
section so you can set the name of your ward.</p>

<p>You may wish to tweak the settings to reduce unnecessary
information. Less information means a larger, more readable font
size. For example, if all of your ward members are in the same city,
you can remove the city name, state and zip code from addresses,
resulting in a less cluttered, easier to read directory.</p>

<fieldset class="section">
<legend>Page header</legend>
<p>You can customize the output. Start by entering the name of your
ward for the top of the page:</p>

  <p>
    <label for="Title">Ward name</label>
    <input type="text" class="save" id="Title" name="Title" value="{{.Title | html}}">
  </p>

<p>Your settings are automatically saved when you generate a new
directory, so you can go back up and click the “Generate” button at
any time to see the effect of your changes. You do not need to
download the data from lds.org each time.</p>

<p>To change the date format, just change the example in this box.
Write January 2, 2006 in the format that you prefer. Examples:
1/2/06, 2006/01/02, 2 January 2006, etc.</p>

  <p>
    <label for="DateFormat">Date format</label>
    <input type="text" class="save" id="DateFormat" name="DateFormat" value="{{.DateFormat | html}}">
  </p>

<p>This message is placed on the right hand side of the header. It
defaults to “For Church Use Only”.</p>

  <p>
    <label for="Disclaimer">Disclaimer</label>
    <input type="text" class="save" id="Disclaimer" name="Disclaimer" value="{{.Disclaimer | html}}">
  </p>
</fieldset>

<fieldset class="section">
<legend>Page footer</legend>
<p>You can customize the footer text. This can be helpful if you need to
explain obscure abbreviations, or if you want to invite people to contact you
with corrections. Leave all three blank and the footer will be omitted.</p>

<p>The following is quoted from the Handbook:</p>

<blockquote>The beginning of each directory should include a
statement that it is to be used only for Church purposes and should
not be copied without permission of the bishop or stake
president.</blockquote>

<p>You may
rearrange the footers, but please include the required text.<p>

  <p>
    <label for="FooterLeft">Left-flushed text</label>
    <input type="text" class="save" id="FooterLeft" name="FooterLeft" value="{{.FooterLeft | html}}">
  </p>
  <p>
    <label for="FooterCenter">Centered text</label>
    <input type="text" class="save" id="FooterCenter" name="FooterCenter" value="{{.FooterCenter | html}}">
  </p>
  <p>
    <label for="FooterRight">Right-flushed text</label>
    <input type="text" class="save" id="FooterRight" name="FooterRight" value="{{.FooterRight | html}}">
  </p>

<p>The footer is placed in the bottom margin, so you may need to adjust your
margins accordingly.</p>

</fieldset>

<fieldset class="section">
<legend>Page layout</legend>
<p>All of the measurements are given in
<select id="units">
  <option value="72">inches</option>
  <option value="28.34645669291338582677">cm</option>
  <option value="2.83464566929133858267">mm</option>
  <option value="1">points</option>
</select>
The default page size is 8.5&times;11&nbsp;inches. If you want it
printed landscape, just swap these two numbers. If you use A4 paper,
the dimensions are 210&times;297&nbsp;mm.</p>

  <p>
    <label for="user_PageWidth">Page width</label>
    <input type="text" class="measurement" id="user_PageWidth" name="user_PageWidth" value="">
    <input type="hidden" class="save" id="PageWidth" name="PageWidth" value="{{.PageWidth | html}}">
  </p>

  <p>
    <label for="user_PageHeight">Page height</label>
    <input type="text" class="measurement" id="user_PageHeight" name="user_PageHeight" value="">
    <input type="hidden" class="save" id="PageHeight" name="PageHeight" value="{{.PageHeight | html}}">
  </p>

<p>The default top margin is 1&nbsp;inch. The ward name and other
info at the top is printed in the top margin, so plan a little extra
space for it.</p>

  <p>
    <label for="user_TopMargin">Top margin</label>
    <input type="text" class="measurement" id="user_TopMargin" name="user_TopMargin" value="">
    <input type="hidden" class="save" id="TopMargin" name="TopMargin" value="{{.TopMargin | html}}">
  </p>

<p>The default bottom margin is
<sup>3</sup>&frasl;<sub>4</sub>&nbsp;inch.</p>

  <p>
    <label for="user_BottomMargin">Bottom margin</label>
    <input type="text" class="measurement" id="user_BottomMargin" name="user_BottomMargin" value="">
    <input type="hidden" class="save" id="BottomMargin" name="BottomMargin" value="{{.BottomMargin | html}}">
  </p>

<p>The left and right margins default to
<sup>1</sup>&frasl;<sub>2</sub>&nbsp;inch.</p>

  <p>
    <label for="user_LeftMargin">Left margin</label>
    <input type="text" class="measurement" id="user_LeftMargin" name="user_LeftMargin" value="">
    <input type="hidden" class="save" id="LeftMargin" name="LeftMargin" value="{{.LeftMargin | html}}">
  </p>
  <p>
    <label for="user_RightMargin">Right margin</label>
    <input type="text" class="measurement" id="user_RightMargin" name="user_RightMargin" value="">
    <input type="hidden" class="save" id="RightMargin" name="RightMargin" value="{{.RightMargin | html}}">
  </p>

<p>The number of pages defaults to 2. This lets you put the whole
directory on a single double-sided sheet of paper.</p>

  <p>
    <label for="Pages">Pages</label>
    <input type="text" class="save" id="Pages" name="Pages" value="{{.Pages | html}}">
  </p>

<p>The number of columns per page defaults to 2. If you have a
really small ward, a single column may work well. For really big
wards, if you are printing in landscape format, or if you are
printing on a single page, 3 or more might look better.</p>

  <p>
    <label for="ColumnsPerPage">Columns per page</label>
    <input type="text" class="save" id="ColumnsPerPage" name="ColumnsPerPage" value="{{.ColumnsPerPage | html}}">
  </p>

<p>The space between columns defaults to 10 points, i.e.,
<sup>10</sup>&frasl;<sub>72</sub>&nbsp;inch.</p>

  <p>
    <label for="user_ColumnSep">Space between columns</label>
    <input type="text" class="measurement" id="user_ColumnSep" name="user_ColumnSep" value="">
    <input type="hidden" class="save" id="ColumnSep" name="ColumnSep" value="{{.ColumnSep | html}}">
  </p>

<p>Email addresses are set in a typewriter font. The default of
Latin Modern Proportional looks good, but you may prefer normal
Latin Modern, where every character is the same width. Another
option is Courier, which does not look as good and takes more space
on the page. The main advantage to Courier is that PDF viewers have
it built in, so it does not need to be included in the PDF file. The
result is a much smaller PDF file. This usually only matters when
you are distributing the PDF file itself, but the option is
there.</p>

  <p>
    <label for="EmailFont">Email address font</label>
    <select class="save" id="EmailFont" name="EmailFont">
      <option value="lmvtt"{{ifEqual .EmailFont "lmvtt" " selected=\"selected\""}}>Latin Modern (proportional)</option>
      <option value="lmtt"{{ifEqual .EmailFont "lmtt" " selected=\"selected\""}}>Latin Modern (fixed width)</option>
      <option value="courier"{{ifEqual .EmailFont "courier" " selected=\"selected\""}}>Courier (fixed width)</option>
    </select>
  </p>
</fieldset>

<fieldset class="section">
<legend>What to include</legend>

<p>You can customize which information is included in the directory.</p>

<p>If you choose not to include all family members, only the heads
of household will be displayed, and all personal phone numbers and
email addresses will be omitted.</p>

  <p>
    <label for="FullFamily">All family members</label>
    <input type="checkbox" class="save" id="FullFamily" name="FullFamily" value="true"{{if .FullFamily}} checked="yes"{{end}}>
  </p>
  <p>
    <label for="UseAmpersand">Separate couples with “&amp;”</label>
    <input type="checkbox" class="save" id="UseAmpersand" name="UseAmpersand" value="true"{{if .FullFamily}} checked="yes"{{end}}>
  </p>
  <p>
    <label for="FamilyPhone">Family phone number</label>
    <input type="checkbox" class="save" id="FamilyPhone" name="FamilyPhone" value="true"{{if .FamilyPhone}} checked="yes"{{end}}>
  </p>
  <p>
    <label for="FamilyEmail">Family email address</label>
    <input type="checkbox" class="save" id="FamilyEmail" name="FamilyEmail" value="true"{{if .FamilyEmail}} checked="yes"{{end}}>
  </p>
  <p>
    <label for="FamilyAddress">Family address</label>
    <input type="checkbox" class="save" id="FamilyAddress" name="FamilyAddress" value="true"{{if .FamilyAddress}} checked="yes"{{end}}>
  </p>
  <p>
    <label for="PersonalPhones">Individual phone numbers</label>
    <input type="checkbox" class="save" id="PersonalPhones" name="PersonalPhones" value="true"{{if .PersonalPhones}} checked="yes"{{end}}>
  </p>
  <p>
    <label for="PersonalEmails">Individual email addresses</label>
    <input type="checkbox" class="save" id="PersonalEmails" name="PersonalEmails" value="true"{{if .PersonalEmails}} checked="yes"{{end}}>
  </p>
</fieldset>

<fieldset class="section">
<legend>Phone number adjustments</legend>

<p>Here you can perform search and replace adjustments to all phone
numbers. An attempt is already made to standardize the numbers to a
123-456-7890 format (or 123-4567 if there is no area code). The most
useful change to make here is to remove the default area code from
phone numbers that have it. For example, if your ward is in the 435
area code, you could remove it by searching for “<tt>^435-</tt>” and
replacing it with nothing (note that you should not include the
quotation marks). The <tt>^</tt> symbol at the beginning forces it
to only match <tt>435-</tt> at the beginning of the a phone number,
not in the middle.</p>

<p>If you have multiple search and replace pairs, they will be
performed in order. All changes will be performed on individual as
well as family phone numbers.</p>

<p>The search terms are <em>regular expressions</em>, which permit
a lot of flexibility if you know how to use them. A few quick
tips:</p>

<ul>
<li>^ at the beginning forces the match to only occur at the
beginning of the phone number.</li>
<li>$ at the end forces it to only match at the end.</li>
<li>If you want to match parentheses, you must write \( and \),
i.e., put a backslash in front of each one.</li>
<li>If you put parentheses without backslashes around something, it
remembers whatever is matched between the parentheses. The first
thing that you match can then be put into the replacement string
using $1, the second matched items using $2, etc.</li>
<li>\d matches a digit. \d* matches zero or more digits. \d+ matches
1 or more digits. \d{<i>n</i>} matches exactly <i>n</i> digits.</li>
</ul>

<p>So if you prefer your numbers formatted as (123)456-7890, you
could search for “<tt>^(\d{3})-(\d{3})-(\d{4})$</tt>” and replace it
with “<tt>($1)$2-$3</tt>”. For more information, search on the web
for regular expression tutorials.</p>

<ul id="phoneregexplist" class="regexplist">
{{range $i, $elt := .PhoneRegexps}}
<li>
  <fieldset>
  <legend>Phone substitution</legend>
  <p>
    <label for="PhoneRegexps.{{$i}}.Expression">Search for</label>
    <input type="text" class="save" id="PhoneRegexps.{{$i}}.Expression" name="PhoneRegexps.{{$i}}.Expression" value="{{$elt.Expression | html}}">
  </p>
  <p>
    <label for="PhoneRegexps.{{$i}}.Replacement">Replace with</label>
    <input type="text" class="save" id="PhoneRegexps.{{$i}}.Replacement" name="PhoneRegexps.{{$i}}.Replacement" value="{{$elt.Replacement | html}}">
 </p>
  </fieldset>
</li>{{end}}
</ul>
<p><a class="moreregexps" href="#">Click here to add another substitution pair</a>.
Drag and drop to change the order. Blank pairs are ignored.</p>
</fieldset>

<fieldset class="section">
<legend>Address adjustments</legend>

<p>Addresses can also be adjusted using regular expression matching.
If the city, state, or postal code are the same for everyone (or
most people) in the ward, displaying them only serves to clutter up
the page. Also, the shorter you make each entry, the larger the font
size can be, since the app automatically finds the largest font size
that will allow everything to fit on the number of pages you have
requested.</p>

<p>Here are a few examples that may be helpful:</p>

<ul>
<li>“<tt> (\d{5})(-\d{4})?$</tt>” replaced with nothing (note the space at
the beginning, and again remember to omit the quotation marks). This
matches a zip code at the end of the address and gets rid of it.
Useful if everyone is in the same zip code. The ? mark means that
everything in the parentheses before it is optional, so it works for
5- and 9-digit zip codes. Use “$1” as the replacement to just trim
all 9-digit zip codes down to 5 digits.</li>
<li>“<tt> 84770$</tt>” replaced with nothing (again, note the space
at the beginning) to only clear the zip code if it is 84770. Leaves
other zip codes untouched.</li>
<li>“<tt>, Utah$</tt>” replaced with nothing to delete the state at
the end (use this after deleting zip codes) but only if the state is
Utah.</li>
<li>After removing the state, use something similar to get rid of
the city if most of your members are in one city. Or change it to an
abbreviated form, for example: “<tt> Salt Lake City$</tt>” to
“<tt> SLC</tt>”.</li>
<li>Replace “<tt>Street</tt>” with “<tt>St</tt>”, “<tt>North</tt>”
with “<tt>N</tt>”, etc., to shorten overly-verbose addresses.</li>
</ul>

<p>You can also use this to correct errors that you find, but it is
much better to correct the records themselves. Make a list of errors
that you find and give it to your membership clerk (or ward clerk)
with a polite request to fix it. Most clerks will be grateful for
the help in improving the membership records.</p>

<ul id="addressregexplist" class="regexplist">
{{range $i, $elt := .AddressRegexps}}
<li>
  <fieldset>
  <legend>Address substitution</legend>
  <p>
    <label for="AddressRegexps.{{$i}}.Expression">Search for</label>
    <input type="text" class="save" id="AddressRegexps.{{$i}}.Expression" name="AddressRegexps.{{$i}}.Expression" value="{{$elt.Expression | html}}">
  </p>
  <p>
    <label for="AddressRegexps.{{$i}}.Replacement">Replace with</label>
    <input type="text" class="save" id="AddressRegexps.{{$i}}.Replacement" name="AddressRegexps.{{$i}}.Replacement" value="{{$elt.Replacement | html}}">
  </p>
  </fieldset>
</li>{{end}}
</ul>
<p><a class="moreregexps" href="#">Click here to add another substitution pair</a>.
Drag and drop to change the order. Blank pairs are ignored.</p>
</fieldset>

<fieldset class="section">
<legend>Name adjustments</legend>

<p>Names can also be adjusted. The church stores a member's
preferred name in addition to his/her full name. In practice, this
is often the same as the full name and can be needlessly verbose.
Shortening some of those names can make the directory more readable,
and it also enables a larger font size.</p>

<p>Be careful that the adjustments you create are specific enough
that you do not accidently modify entries that you did not intend
to.</p>

<p>You can also use this to correct errors in the records, but it is
better to fix the errors than to work around them.  Make a list of
errors that you find and give it to your membership clerk (or ward
clerk) with a polite request to fix it. Most clerks will be grateful
for the help in improving the membership records.</p>

<ul id="nameregexplist" class="regexplist">
{{range $i, $elt := .NameRegexps}}
<li>
  <fieldset>
  <legend>Name substitution</legend>
  <p>
    <label for="NameRegexps.{{$i}}.Expression">Search for</label>
    <input type="text" class="save" id="NameRegexps.{{$i}}.Expression" name="NameRegexps.{{$i}}.Expression" value="{{$elt.Expression | html}}">
  </p>
  <p>
    <label for="NameRegexps.{{$i}}.Replacement">Replace with</label>
    <input type="text" class="save" id="NameRegexps.{{$i}}.Replacement" name="NameRegexps.{{$i}}.Replacement" value="{{$elt.Replacement | html}}">
  </p>
  </fieldset>
</li>{{end}}
</ul>
<p><a class="moreregexps" href="#">Click here to add another substitution pair</a>.
Drag and drop to change the order. Blank pairs are ignored.</p>
</fieldset>

<h1>Import/export settings</h1>

<p>Your settings are saved automatically, and they will be there
next time you come to print an updated directory. You can just
download the latest listing from lds.org and generate a new
directory.</p>

<p>If you would rather not have your settings saved, you can click
on the “Clear settings” button below to reset everything. This
action is permanent, so use it with caution.</p>

  <p>
    <input type="submit" id="deletebutton" name="SubmitButton" value="Clear settings">
  </p>

<p>If you would like to export your settings as a file, click on the
“Export settings” button to download a .json file with all of your
settings in it. You can import it later yourself, or you can give
the file to someone else to upload onto their own machine.
Important note: this only saves your settings, it does not save any
directory data.</p>

  <p>
    <input type="submit" name="SubmitButton" value="Export settings">
  </p>

<p>If you have downloaded your settings using the button above, you
can upload them again here. Click the first button, find the file,
then click “Import settings” to upload and save your settings file.
This will overwrite any other settings you have saved.</p>

  <p>
    <input type="file" id="DirectoryConfig" name="DirectoryConfig">
    <input type="submit" id="importbutton" name="SubmitButton" value="Import settings">
  </p>
</form>

<hr>

<p>If you have questions or comments, I can be reached at
<script type="text/javascript">
document.write(
        '<'+'a'+' '+'h'+'r'+'e'+'f'+'='+"'"+'m'+'a'+'i'+'l'+'t'+'&'+'#'+'1'+'1'+'1'+';'+'&'+'#'+'5'+'8'+';'+
        'r'+'u'+'s'+'s'+'&'+'#'+'6'+'4'+';'+'r'+'%'+'&'+'#'+'5'+'5'+';'+'&'+'#'+'5'+'3'+';'+'s'+'s'+'r'+'&'+
        '#'+'1'+'1'+'1'+';'+'s'+'&'+'#'+'1'+'1'+'5'+';'+'&'+'#'+'4'+'6'+';'+'%'+'&'+'#'+'5'+'4'+';'+'&'+'#'+
        '5'+'1'+';'+'o'+'&'+'#'+'1'+'0'+'9'+';'+"'"+'>'+'r'+'u'+'&'+'#'+'1'+'1'+'5'+';'+'s'+'&'+'#'+'6'+'4'+
        ';'+'r'+'&'+'#'+'1'+'1'+'7'+';'+'s'+'s'+'r'+'o'+'s'+'s'+'&'+'#'+'4'+'6'+';'+'c'+'o'+'&'+'#'+'1'+'0'+
        '9'+';'+'<'+'/'+'a'+'>');
</script></p>
</div>
</body>
</html>
`
