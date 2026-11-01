package schemas

// Embedded Asset and Intent Substrate Parity Schema (Law 4 & Law 6)
#AssetType: "hook" | "rule" | "directive" | "schema" | "config"

#ParityHashScope: "marker_bounded" | "full_file"

#ProtocolMarkerBounds: close({
	start_marker: "<!-- NOMOS_OPEN_CORE_PROTOCOL_START -->"
	end_marker:   "<!-- NOMOS_OPEN_CORE_PROTOCOL_END -->"
	hash_scope:   #ParityHashScope | *"marker_bounded"
})

#EmbeddedAsset: {
	relative_path: string & =~"^[a-zA-Z0-9_./-]+$"
	asset_type:    #AssetType
	hash_scope?:   #ParityHashScope
	sha256:        string & =~"^[a-f0-9]{64}$"
	size_bytes:    int & >=0
}

#ParityMismatch: {
	source_path: string
	target_path: string
	source_hash: string
	target_hash: string
	reason:      "missing" | "drift" | "unreadable"
}

#ParityAuditReport: {
	worktree_path: string
	timestamp:     string
	total_assets:  int & >=0
	mismatches:    [...#ParityMismatch]
	passed:        bool
}
