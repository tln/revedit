# revedit - demo/toy reverse editing utility for Go templates

revedit allows you to:

1. Run Go templates like so:

   revedit demo.tmpl
   
2. Apply the edits back to the input templates

   # $EDITOR demo.html
   # revedit demo.html
   
 html/template and text/template are copied from Go 1.1; they have been modified to record the origination of every piece of output.