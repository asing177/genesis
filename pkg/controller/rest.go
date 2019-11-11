/*
	Copyright 2019 whiteblock Inc.
	This file is a part of the genesis.

	Genesis is free software: you can redistribute it and/or modify
    it under the terms of the GNU General Public License as published by
    the Free Software Foundation, either version 3 of the License, or
    (at your option) any later version.

    Genesis is distributed in the hope that it will be useful,
    but WITHOUT ANY WARRANTY; without even the implied warranty of
    MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
    GNU General Public License for more details.

    You should have received a copy of the GNU General Public License
    along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

package controller

import (
	log "github.com/sirupsen/logrus"
	"github.com/whiteblock/genesis/pkg/entity"
	"github.com/whiteblock/genesis/pkg/handler"
	"github.com/whiteblock/genesis/pkg/helper"
	"net/http"
	"strings"
)

//RestController handles the REST API server
type RestController interface {
	//Start attempts to start the server
	Start()
}

type restController struct {
	conf entity.RestConfig
	hand handler.RestHandler
	mux  helper.Router
}

//NewRestController creates a new rest controller
func NewRestController(conf entity.RestConfig, hand handler.RestHandler, mux helper.Router) RestController {
	log.Debug("creating a new rest controller")
	return &restController{conf: conf, hand: hand, mux: mux}
}

// Start starts the rest server, blocking the calling thread from returning
func (rc restController) Start() {

	rc.mux.HandleFunc("/command", rc.hand.AddCommands).Methods("POST")
	rc.mux.HandleFunc("/health", rc.hand.HealthCheck).Methods("GET")

	log.WithFields(log.Fields{"socket": rc.conf.Listen}).Info("listening for requests")
	log.Fatal(http.ListenAndServe(rc.conf.Listen, removeTrailingSlash(rc.mux)))
}

func removeTrailingSlash(next http.Handler) http.Handler { //TODO middleware
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = strings.TrimSuffix(r.URL.Path, "/")
		next.ServeHTTP(w, r)
	})
}