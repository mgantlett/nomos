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

;; Anti-shim stub and test-skip patterns
[(pass_statement) @shim.stub
 (raise_statement) @shim.stub
 (decorator) @test.skip]
