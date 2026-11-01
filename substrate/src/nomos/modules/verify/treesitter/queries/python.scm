;; Function and method definitions
((comment)* @func.doc
 .
 (function_definition name: (identifier) @func.name body: (block) @func.body))

;; Complexity decision branch nodes
[(if_statement)
 (for_statement)
 (while_statement)
 (except_clause)
 (boolean_operator)] @branch
