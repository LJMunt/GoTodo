package config

import (
	"context"
	"net/http"
	"time"
)

type ConfigStatusResponse struct {
	Auth struct {
		AllowSignup              bool `json:"allowSignup"`
		RequireEmailVerification bool `json:"requireEmailVerification"`
		AllowReset               bool `json:"allowReset"`
		AllowTOTP                bool `json:"allowTOTP"`
	} `json:"auth"`
	Instance struct {
		ReadOnly bool `json:"readOnly"`
	} `json:"instance"`
	Features struct {
		Organizations bool `json:"organizations"`
	} `json:"features"`
}

func GetConfigStatusHandler(db configQuerier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		var status ConfigStatusResponse

		rows, err := db.Query(ctx, `
			SELECT key, value_json 
			FROM config_keys 
			WHERE key IN ('auth.allowSignup', 'auth.requireEmailVerification', 'auth.allowReset', 'auth.allowTOTP', 'instance.readOnly', 'features.organizations')
		`)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to fetch status")
			return
		}
		defer rows.Close()

		for rows.Next() {
			var key, val string
			if err := rows.Scan(&key, &val); err != nil {
				continue
			}

			switch key {
			case "auth.allowSignup":
				status.Auth.AllowSignup = castConfigValue(val, "boolean").(bool)
			case "auth.requireEmailVerification":
				status.Auth.RequireEmailVerification = castConfigValue(val, "boolean").(bool)
			case "auth.allowReset":
				status.Auth.AllowReset = castConfigValue(val, "boolean").(bool)
			case "auth.allowTOTP":
				status.Auth.AllowTOTP = castConfigValue(val, "boolean").(bool)
			case "instance.readOnly":
				status.Instance.ReadOnly = castConfigValue(val, "boolean").(bool)
			case "features.organizations":
				status.Features.Organizations = castConfigValue(val, "boolean").(bool)
			}
		}

		writeJSON(w, http.StatusOK, status)
	}
}
