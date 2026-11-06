;; Method and singleton method definitions
((comment)* @func.doc
 .
 [(method name: (identifier) @func.name body: (body_statement) @func.body)
  (singleton_method name: (identifier) @func.name body: (body_statement) @func.body)])

;; Complexity decision branch nodes
[(if)
 (unless)
 (elsif)
 (if_modifier)
 (unless_modifier)
 (case)
 (when)
 (while)
 (until)
 (for)
 (rescue)
 (conditional)
 (binary)] @branch

;; Anti-shim stub and test-skip patterns
[(call) @shim.stub
 (call) @test.skip]
