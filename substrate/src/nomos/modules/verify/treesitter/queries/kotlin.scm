;; Function declarations in Kotlin
((multiline_comment)* @func.doc
 .
 (function_declaration (simple_identifier) @func.name (function_body) @func.body))

;; Complexity decision branch nodes
[(if_expression)
 (when_entry)
 (for_statement)
 (while_statement)
 (do_while_statement)
 (catch_block)
 (conjunction_expression)
 (disjunction_expression)
 (elvis_expression)] @branch

;; Anti-shim stub patterns (Gate 10)
[(call_expression (simple_identifier) @shim.stub)]

;; Test skip / reward-hacking annotations
[(annotation (user_type (type_identifier) @test.skip))]
