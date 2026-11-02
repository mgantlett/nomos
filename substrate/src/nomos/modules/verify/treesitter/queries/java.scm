;; Method and constructor declarations in Java
((block_comment)* @func.doc
 .
 [(method_declaration name: (identifier) @func.name body: (block) @func.body)
  (constructor_declaration name: (identifier) @func.name body: (constructor_body) @func.body)])

;; Complexity decision branch nodes
[(if_statement)
 (for_statement)
 (enhanced_for_statement)
 (while_statement)
 (do_statement)
 (switch_label)
 (catch_clause)
 (ternary_expression)
 (binary_expression operator: "&&")
 (binary_expression operator: "||")] @branch

;; Anti-shim stub patterns (Gate 10)
[(throw_statement (object_creation_expression type: (type_identifier) @shim.stub))]

;; Test skip / reward-hacking annotations
[(marker_annotation name: (identifier) @test.skip)]
