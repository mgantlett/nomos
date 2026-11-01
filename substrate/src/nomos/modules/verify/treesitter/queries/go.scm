;; Function and Method declarations with preceding comments
((comment)* @func.doc
 .
 [(function_declaration name: (identifier) @func.name body: (block) @func.body)
  (method_declaration name: (field_identifier) @func.name body: (block) @func.body)])

;; Complexity decision branch nodes
[(if_statement)
 (for_statement)
 (expression_case_clause)
 (type_case_clause)
 (communication_case_clause)
 (binary_expression operator: "&&")
 (binary_expression operator: "||")] @branch
