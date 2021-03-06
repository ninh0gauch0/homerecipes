package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/leemcloughlin/logfile"
	"github.com/ninh0gauch0/hrstypes"
)

var (
	logFileOn = true
	logFile   *logfile.LogFile
)

const (
	// DECODEERROR Constant
	DECODEERROR = "Failed validation"
	// FATALERROR Constant
	FATALERROR = "Fatal error"
)

// Init the configuration needed to start the server
func (s *Server) Init() bool {

	// Init logfile
	logFile, err := logfile.New(
		&logfile.LogFile{
			FileName: "homeRecipesServer.log",
			MaxSize:  1000 * 1024,
			Flags:    logfile.FileOnly | logfile.RotateOnStart})
	if err != nil {
		s.logger.Errorf("Failed to create log file %s: %s", "logFileName", err.Error())
		logFileOn = false
	}
	log.SetOutput(logFile)

	// init router
	s.router = mux.NewRouter()

	s.initialized = true
	return true
}

// Start the server
func (s *Server) Start(config map[string]string) chan bool {

	// Recovering config server
	addr, ok := config["addr"]
	if !ok {
		addr = ":8080"
	}
	s.Addr = addr

	if s.initialized != true {
		err := s.Init()
		if err {
			return nil
		}
	}

	s.logger.Infof("Starting server....")

	s.worker = &Worker{}
	s.worker.Init(s.Ctx, s.GetLogger())

	s.addRoutes()

	exitChan := make(chan bool)

	// Go routines and channel to orchestrate
	go func() {
		<-exitChan
		s.logger.Infoln("Stopping server")
		// Server shutdown
		err := s.Server.Shutdown(s.Ctx)

		if err != nil {
			s.customErrorLogger("Error shutdowning server - error: %s", err.Error())
		}

		// CLose the logfile
		logFile.Close()
	}()
	go func() {
		log.Printf("Listening on... %s", s.Addr)
		log.Fatal(http.ListenAndServe(addr, s.router))
	}()

	return exitChan
}

// addRoutes - Define API routes
func (s *Server) addRoutes() {
	hrsRoutes := s.router.PathPrefix("/hrs").Subrouter()

	/** RECIPES ENDPOINTS**/
	hrsRoutes.HandleFunc("/recipes", func(w http.ResponseWriter, r *http.Request) {
		s.logger.Debugln("creating recipe...")

		var recipe hrstypes.Recipe
		var data []byte
		var err error

		status := http.StatusCreated
		hrsResp := initResponse()

		decoder := json.NewDecoder(r.Body)
		err = decoder.Decode(&recipe)
		defer r.Body.Close()

		if err != nil {
			decodeError(&hrsResp, &data, &err)
			status = http.StatusConflict
		} else {
			hrsResp = s.worker.CreateRecipe(&recipe)

			data, err = json.Marshal(hrsResp)

			if err != nil {
				status = http.StatusConflict
				s.customErrorLogger("Json marshaling error - error: %s", err.Error())
				marshallError(&hrsResp, &data, &err)
			} else {
				if hrsResp.Error != nil {
					s.customErrorLogger(hrsResp.Error.ShowError())
					status = hrsResp.Status.Code
				} else {
					s.customInfoLogger("Recipe created:\n%s", hrsResp.RespObj.GetObjectInfo())
				}

			}
		}

		w.WriteHeader(status)
		w.Write(data)
	}).Methods("POST")

	hrsRoutes.HandleFunc("/recipes/{id}", func(w http.ResponseWriter, r *http.Request) {
		s.logger.Debugln("searching recipe...")
		status := http.StatusOK

		vars := mux.Vars(r)
		id := vars["id"]

		hrsResp := s.worker.GetRecipeByID(id)

		data, err := json.Marshal(hrsResp)

		if err != nil {
			status = http.StatusConflict
			s.customErrorLogger("Json marshaling error - error: %s", err.Error())
			marshallError(&hrsResp, &data, &err)
		} else {
			if hrsResp.Error != nil {
				s.customErrorLogger(hrsResp.Error.ShowError())
				status = hrsResp.Status.Code
			} else {
				if hrsResp.Error != nil {
					s.customErrorLogger(hrsResp.Error.ShowError())
					status = hrsResp.Status.Code
				} else {
					s.customInfoLogger("Recipe returned:\n%s", hrsResp.RespObj.GetObjectInfo())
				}
			}
		}

		w.WriteHeader(status)
		w.Write(data)
	}).Methods("GET")

	hrsRoutes.HandleFunc("/recipes/{id}", func(w http.ResponseWriter, r *http.Request) {
		s.logger.Debugln("patchting recipe...")
		var data []byte
		var recipe hrstypes.Recipe

		status := http.StatusOK
		vars := mux.Vars(r)
		id := vars["id"]
		hrsResp := initResponse()
		decoder := json.NewDecoder(r.Body)
		defer r.Body.Close()

		err := decoder.Decode(&recipe)

		if err != nil {
			decodeError(&hrsResp, &data, &err)
			status = http.StatusConflict
		} else {
			hrsResp = s.worker.PatchRecipeByID(id, &recipe)
			data, err = json.Marshal(hrsResp)

			if err != nil {
				s.customErrorLogger("Json marshaling error - error: %s", err.Error())
				marshallError(&hrsResp, &data, &err)
				status = http.StatusConflict
			} else {
				if hrsResp.Error != nil {
					s.customErrorLogger(hrsResp.Error.ShowError())
					status = hrsResp.Status.Code
				} else {
					s.customInfoLogger("Recipe patched:\n%s", hrsResp.RespObj.GetObjectInfo())
				}
			}
		}

		w.WriteHeader(status)
		w.Write(data)
	}).Methods("PATCH")

	hrsRoutes.HandleFunc("/recipes/{id}", func(w http.ResponseWriter, r *http.Request) {
		s.logger.Debugln("deleting recipe...")
		status := http.StatusNoContent
		vars := mux.Vars(r)
		id := vars["id"]

		hrsResp := s.worker.DeleteRecipe(id)
		data, err := json.Marshal(hrsResp)

		if err != nil {
			s.customErrorLogger("Json marshaling error - error: %s", err.Error())
			marshallError(&hrsResp, &data, &err)
			status = http.StatusConflict
		} else {
			if hrsResp.Error != nil {
				s.customErrorLogger(hrsResp.Error.ShowError())
				status = hrsResp.Status.Code
			} else {
				s.customInfoLogger("Recipe deleted")
			}
		}

		w.WriteHeader(status)
		w.Write(data)
	}).Methods("DELETE")

	/** INGREDIENTS ENDPOINTS **/
	hrsRoutes.HandleFunc("/ingredients", func(w http.ResponseWriter, r *http.Request) {
		s.logger.Debugln("creating ingredients...")

		var data []byte
		var err error

		status := http.StatusCreated
		hrsResp := initResponse()

		decoder := json.NewDecoder(r.Body)
		var ingredient hrstypes.Ingredient
		err = decoder.Decode(&ingredient)

		if err != nil {
			decodeError(&hrsResp, &data, &err)
			status = http.StatusConflict
		} else {
			hrsResp := s.worker.CreateIngredient(&ingredient)
			data, err = json.Marshal(hrsResp)

			if err != nil {
				s.customErrorLogger("Json marshaling error - error: %s", err.Error())
				marshallError(&hrsResp, &data, &err)
				status = http.StatusConflict
			} else {
				if hrsResp.Error != nil {
					s.customErrorLogger(hrsResp.Error.ShowError())
					status = hrsResp.Status.Code
				} else {
					s.customInfoLogger("Ingredient created:\n%s", hrsResp.RespObj.GetObjectInfo())
				}
			}
		}
		defer r.Body.Close()

		w.WriteHeader(status)
		w.Write(data)
	}).Methods("POST")

	hrsRoutes.HandleFunc("/ingredients/{id}", func(w http.ResponseWriter, r *http.Request) {
		s.logger.Debugln("searching ingredients...")
		status := http.StatusOK
		vars := mux.Vars(r)
		id := vars["id"]

		hrsResp := s.worker.GetIngredientByID(id)
		data, err := json.Marshal(hrsResp)

		if err != nil {
			s.customErrorLogger("Json marshaling error - error: %s", err.Error())
			marshallError(&hrsResp, &data, &err)
			status = http.StatusConflict
		} else {
			if hrsResp.Error != nil {
				s.customErrorLogger(hrsResp.Error.ShowError())
				status = hrsResp.Status.Code
			} else {
				s.customInfoLogger("Ingredient returned:\n%s", hrsResp.RespObj.GetObjectInfo())
			}
		}

		w.WriteHeader(status)
		w.Write(data)
	}).Methods("GET")

	hrsRoutes.HandleFunc("/ingredients/{id}", func(w http.ResponseWriter, r *http.Request) {
		s.logger.Debugln("patching ingredients...")
		var data []byte
		var ingredient hrstypes.Ingredient

		status := http.StatusOK
		hrsResp := initResponse()
		vars := mux.Vars(r)
		id := vars["id"]
		decoder := json.NewDecoder(r.Body)
		defer r.Body.Close()

		err := decoder.Decode(&ingredient)

		if err != nil {
			decodeError(&hrsResp, &data, &err)
			status = http.StatusConflict
		} else {
			hrsResp = s.worker.PatchIngredientByID(id, &ingredient)
			data, err = json.Marshal(hrsResp)

			if err != nil {
				s.customErrorLogger("Json marshaling error - error: %s", err.Error())
				marshallError(&hrsResp, &data, &err)
				status = http.StatusConflict
			} else {
				if hrsResp.Error != nil {
					s.customErrorLogger(hrsResp.Error.ShowError())
					status = hrsResp.Status.Code
				} else {
					s.customInfoLogger("Ingredient modified:\n%s", hrsResp.RespObj.GetObjectInfo())
				}
			}
		}

		w.WriteHeader(status)
		w.Write(data)
	}).Methods("PATCH")

	hrsRoutes.HandleFunc("/ingredients/{id}", func(w http.ResponseWriter, r *http.Request) {
		s.logger.Debugln("deleting ingredient...")
		status := http.StatusNoContent
		vars := mux.Vars(r)
		id := vars["id"]

		hrsResp := s.worker.DeleteIngredient(id)
		data, err := json.Marshal(hrsResp)

		if err != nil {
			s.customErrorLogger("Json marshaling error - error: %s", err.Error())
			marshallError(&hrsResp, &data, &err)
			status = http.StatusConflict
		} else {
			if hrsResp.Error != nil {
				s.customErrorLogger(hrsResp.Error.ShowError())
				status = hrsResp.Status.Code
			} else {
				s.customInfoLogger("Ingredient deleted")
			}
		}

		w.WriteHeader(status)
		w.Write(data)
	}).Methods("DELETE")

	/** OTHER ENDPOINTS **/
	hrsRoutes.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "WTF\n")
	}).Methods("GET")
}

/** PRIVATE METHODS **/

func initResponse() hrstypes.HRAResponse {
	resp := hrstypes.HRAResponse{}
	return resp
}

func fatalResponse(err error) hrstypes.HRAResponse {
	status := hrstypes.Status{
		Code:        http.StatusConflict,
		Description: FATALERROR,
	}
	hrsError := hrstypes.FatalError{}
	hrsError.SetError(err.Error())
	resp := hrstypes.HRAResponse{
		Status: status,
		Error:  &hrsError,
	}

	return resp
}

func decodeError(hrsResp *hrstypes.HRAResponse, data *[]byte, err *error) {
	errRsp := initResponse()
	errRsp.Status = hrstypes.Status{
		Code:        http.StatusConflict,
		Description: DECODEERROR,
	}
	*data, *err = json.Marshal(errRsp)

	if err != nil {
		*hrsResp = fatalResponse(*err)
		*data, *err = json.Marshal(hrsResp)
	}
}

func marshallError(hrsResp *hrstypes.HRAResponse, data *[]byte, err *error) {
	*hrsResp = fatalResponse(*err)
	*data, *err = json.Marshal(hrsResp)

	if err != nil {
		*hrsResp = hrstypes.HRAResponse{}
	}
}

// CustomErrorLogger - Writes error
func (s *Server) customErrorLogger(msg string, args ...interface{}) {
	MSG := "[ERROR] " + msg

	if logFileOn {
		log.Printf(MSG, args)
	}
}

// customInfoLogger - Writes info
func (s *Server) customInfoLogger(msg string, args ...interface{}) {
	MSG := "[INFO] " + msg

	if logFileOn {
		log.Printf(MSG, args)
	}
}
