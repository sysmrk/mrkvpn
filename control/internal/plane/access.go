package plane

import (
	"net/http"
	"sort"
	"time"
)

func (a *API) tokens(w http.ResponseWriter,r *http.Request,actor Token) {
	if r.Method=="GET" {
		st:=a.store.snapshot()
		out:=make([]TokenView,0,len(st.Tokens))
		for _,v:=range st.Tokens {out=append(out,v.View())}
		sort.Slice(out,func(i,j int)bool{return out[i].CreatedAt.After(out[j].CreatedAt)})
		writeJSON(w,200,map[string]any{"items":out})
		return
	}
	var input struct {
		Name string `json:"name"`
		Role string `json:"role"`
		ExpiresHours int `json:"expires_hours"`
	}
	if err:=decodeJSON(w,r,&input);err!=nil{respondError(w,err);return}
	if !validName(input.Name,80)||input.ExpiresHours<1||input.ExpiresHours>2160{
		respondError(w,invalid("name and expires_hours (1..2160) required"));return
	}
	switch input.Role {case "owner","admin","operator","auditor":default:respondError(w,invalid("unsupported role"));return}
	raw,id,hash,err:=newSecret()
	if err!=nil{respondError(w,err);return}
	now:=time.Now().UTC()
	t:=Token{ID:id,Name:input.Name,Role:input.Role,Digest:hash,CreatedAt:now,ExpiresAt:now.Add(time.Duration(input.ExpiresHours)*time.Hour)}
	out,err:=a.store.change(actor.ID,"tokens.create",id,func(st *State)(any,error){
		if len(st.Tokens)>=100{return nil,conflict("token inventory capacity exceeded")}
		st.Tokens[id]=t
		return map[string]any{"token":raw,"metadata":t.View(),"warning":"Shown once. Save securely; only SHA-256 digest is stored."},nil
	})
	if err!=nil{respondError(w,err);return}
	writeJSON(w,http.StatusCreated,out)
}
func (a *API) revokeToken(w http.ResponseWriter,r *http.Request,actor Token){
	id:=r.PathValue("id")
	out,err:=a.store.change(actor.ID,"tokens.revoke",id,func(st *State)(any,error){
		t,ok:=st.Tokens[id]
		if !ok{return nil,missing()}
		if t.Revoked{return nil,conflict("token already revoked")}
		if t.Role=="owner" {
			owners:=0
			for _,candidate:=range st.Tokens {
				if candidate.Role=="owner"&&!candidate.Revoked&&(candidate.ExpiresAt.IsZero()||candidate.ExpiresAt.After(time.Now().UTC())){owners++}
			}
			if owners<=1{return nil,conflict("cannot revoke the last active owner token")}
		}
		t.Revoked=true
		st.Tokens[id]=t
		return t.View(),nil
	})
	if err!=nil{respondError(w,err);return}
	writeJSON(w,200,out)
}
