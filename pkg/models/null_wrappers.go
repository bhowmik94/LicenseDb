// SPDX-FileCopyrightText: 2023 Siemens AG
// SPDX-FileContributor: Dearsh Oberoi <oberoidearsh@gmail.com>
//
// SPDX-License-Identifier: GPL-2.0-only

package models

import (
	"database/sql"
	"encoding/json"
)

// NullString is wrapper over sql.NullString for having custom marshalling and
// unmarshalling of sql.NullString to json
type NullString struct {
	sql.NullString
}

func (v NullString) MarshalJSON() ([]byte, error) {
	if v.Valid {
		return json.Marshal(v.String)
	}
	return json.Marshal(nil)
}

func (v *NullString) UnmarshalJSON(data []byte) error {
	var x *string
	if err := json.Unmarshal(data, &x); err != nil {
		return err
	}
	if x == nil || *x == "" {
		v.Valid = false
	} else {
		v.Valid = true
		v.String = *x
	}
	return nil
}
