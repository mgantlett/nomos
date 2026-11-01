;; Function and method definitions
((comment)* @func.doc
 .
 (function_definition declarator: (function_declarator declarator: (identifier) @func.name) body: (compound_statement) @func.body))

;; Complexity decision branch nodes
[(if_statement)
 (for_statement)
 (while_statement)
 (switch_statement)
 (do_statement)
 (catch_clause)] @branch
