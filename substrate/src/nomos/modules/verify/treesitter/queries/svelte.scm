;; Svelte script blocks and components
((comment)* @func.doc
 .
 [(script_element)
  (element)
  (function_declaration name: (identifier) @func.name)])

;; Complexity decision branch nodes
[(if_statement)
 (for_statement)
 (while_statement)
 (each_block)
 (await_block)
 (binary_expression operator: "&&")
 (binary_expression operator: "||")] @branch
