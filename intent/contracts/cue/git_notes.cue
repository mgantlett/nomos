package schemas

// Git Notes Cryptographic SSoT and Ephemeral Projection Contract (Law 4 & Law 6)
#ReleaseNotePayload: close({
	task_key:            #TaskKey
	commit_sha:          string & =~"^[a-f0-9]{40}$"
	timestamp:           string
	phase:               "SYNC"
	dod_gates_passed:    int & >= 42
	capbac_token_id:     string
	walkthrough_content: string
	sha256_receipt:      string & =~"^[a-f0-9]{64}$"
})

