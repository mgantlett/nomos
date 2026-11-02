;; Method and constructor declarations in C#
((comment)* @func.doc
 .
 [(method_declaration name: (identifier) @func.name body: (block) @func.body)
  (constructor_declaration name: (identifier) @func.name body: (block) @func.body)])

;; Complexity decision branch nodes
[(if_statement)
 (for_statement)
 (foreach_statement)
 (while_statement)
 (do_statement)
 (switch_section)
 (catch_clause)
 (conditional_expression)
 (binary_expression operator: "&&")
 (binary_expression operator: "||")
 (binary_expression operator: "??")] @branch

;; Anti-shim stub patterns (Gate 10)
[(throw_statement (object_creation_expression type: (identifier) @shim.stub))]

;; Test skip / reward-hacking attributes
[(attribute name: (identifier) @test.skip)]
