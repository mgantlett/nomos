package schemas

import "strings"

// Role defines the allowed agent or user personas.
#Role: "architect" | "developer" | "observer" | "guest" | "critic"

// Action defines the authorized execution operations.
#Action: "read" | "write" | "test" | "verify" | "release"

// AuthzPolicy defines the authoritative authorization evaluation contract.
#AuthzPolicy: close({
	role:     #Role
	action:   #Action
	resource: string & strings.MinRunes(1) & =~"^\\S+$"
	allow:    bool
})

role:     #Role | *"guest"
action:   #Action | *"read"
resource: (string & strings.MinRunes(1) & =~"^\\S+$") | *""

allow: *false | bool

// Deterministic Least-Privilege Authorization Predicates (Law 7)
if resource != "" {
	if role == "architect" {
		allow: true
	}
	if role == "developer" && (action == "read" || action == "write" || action == "test") {
		allow: true
	}
	if (role == "observer" || role == "guest") && action == "read" {
		allow: true
	}
	if role == "critic" && (action == "read" || action == "verify") {
		allow: true
	}
}
