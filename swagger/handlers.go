package swagger

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

type secret struct {
	Secret    string    `json:"secretText"`
	Hash      string    `json:"hash"`
	CreatedAt time.Time `json:"createdAt"`
	ExpiredAt time.Time `json:"expiresAt"`
	Remaining int32     `json:"remainingViews"`
}

func getSecret(hash string) (secret, error) {
	secr := secret{}
	rows, err := db.Query("select * from public.secret where hash=$1 ", hash)
	if err != nil {
		return secr, err
	}
	defer rows.Close()
	for rows.Next() {
		err := rows.Scan(
			&secr.Secret,
			&secr.Hash,
			&secr.CreatedAt,
			&secr.ExpiredAt,
			&secr.Remaining,
		)
		if err != nil {
			return secr, err
		}
	}

	err = rows.Err()
	return secr, err
}

func GetSecretByHash(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	secret, err := getSecret(vars["hash"])

	if err != nil || secret.Secret == "" {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "Secret not found")
		return
	}

	if secret.Remaining == 0 {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "No more remaining view")
		return
	}

	if diff := secret.ExpiredAt.Sub(time.Now()); diff < 0 {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "Secret is expired")
		return
	}

	secret.Remaining--

	sqlStatement := `
	Update public.secret  set  remaining = $1`
	db.Exec(sqlStatement, secret.Remaining)

	w.Header().Set("Content-Type", "application/json")
	jData, err := json.Marshal(secret)
	if err != nil {
		// handle error
	}
	fmt.Fprintf(w, string(jData))
}

type SecretPostPayload struct {
	Secret          string `json:"secret"`
	ExpireAfterView int32  `json:"expireAfterView"`
	ExpireAfter     int32  `json:"expireAfter"`
}

func AddSecret(w http.ResponseWriter, r *http.Request) {
	// Read body
	b, err := ioutil.ReadAll(r.Body)
	defer r.Body.Close()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	// Unmarshal
	var msg SecretPostPayload
	err = json.Unmarshal(b, &msg)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	hash := md5.Sum([]byte(msg.Secret))

	minutes, _ := time.ParseDuration(fmt.Sprintf("%sm,", msg.ExpireAfter))
	expired := time.Now().Add(minutes)

	sqlStatement := `
	INSERT INTO public.secret  (secret, hash, expired,remaining)
	VALUES ($1, $2, $3, $4)`
	db.Exec(sqlStatement, msg.Secret, fmt.Sprintf("%x", hash), expired, msg.ExpireAfterView)

	w.Header().Set("content-type", "application/json")

	secret := secret{
		Secret:    msg.Secret,
		Hash:      fmt.Sprintf("%x", hash),
		CreatedAt: time.Now(),
		ExpiredAt: expired,
		Remaining: msg.ExpireAfterView,
	}
	jData, err := json.Marshal(secret)
	if err != nil {
		// handle error
	}
	fmt.Fprintf(w, string(jData))

}
