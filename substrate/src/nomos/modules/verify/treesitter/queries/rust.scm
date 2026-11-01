;; Function and method definitions
((line_comment)* @func.doc
 .
 (function_item name: (identifier) @func.name body: (block) @func.body))

;; Complexity decision branch nodes
[(if_expression)
 (match_arm)
 (loop_expression)
 (for_expression)
 (while_expression)] @branch
