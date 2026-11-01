;; Function declarations, methods, and arrow functions
((comment)* @func.doc
 .
 [(function_declaration name: (identifier) @func.name body: (statement_block) @func.body)
  (method_definition name: (property_identifier) @func.name body: (statement_block) @func.body)
  (lexical_declaration (variable_declarator name: (identifier) @func.name value: (arrow_function body: (statement_block) @func.body)))])

;; Complexity decision branch nodes
[(if_statement)
 (for_statement)
 (for_in_statement)
 (while_statement)
 (switch_case)
 (catch_clause)
 (ternary_expression)
 (binary_expression operator: "&&")
 (binary_expression operator: "||")
 (binary_expression operator: "??")] @branch
