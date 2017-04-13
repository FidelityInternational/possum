package webServer_test

import (
	"bytes"
	"database/sql"
	"fmt"
	"github.com/DATA-DOG/go-sqlmock"
	webs "github.com/FidelityInternational/possum/web_server"
	"github.com/gorilla/mux"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
)

var (
	fakeServer1 *httptest.Server
	fakeServer2 *httptest.Server
)

func Router(controller *webs.Controller) *mux.Router {
	server := &webs.Server{Controller: controller}
	r := server.Start()
	return r
}

func init() {
	var controller *webs.Controller
	http.Handle("/", Router(controller))
}

func mockFailedStateDBConn(driverName string, connectionString string) (*sql.DB, error) {
	db, mock, err := sqlmock.New()
	if err != nil {
		fmt.Printf("\nan error '%s' was not expected when opening a stub database connection\n", err)
		os.Exit(1)
	}
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS state.").WillReturnError(fmt.Errorf("An error has occurred: %s", "Database Create Error"))
	return db, err
}

func mockDBConn(driverName string, connectionString string) (*sql.DB, error) {
	db, mock, err := sqlmock.New()
	if err != nil {
		fmt.Printf("\nan error '%s' was not expected when opening a stub database connection\n", err)
		os.Exit(1)
	}
	mRows := sqlmock.NewRows([]string{"possum", "state"}).
		AddRow("mother", "alive")
	fRows := sqlmock.NewRows([]string{"possum", "state"}).
		AddRow("father", "alive")
	jRows := sqlmock.NewRows([]string{"possum", "state"}).
		AddRow("joey", "alive")

	mock.ExpectExec("CREATE TABLE IF NOT EXISTS state.*").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery("SELECT (.+) FROM state WHERE possum=").WillReturnRows(mRows)
	mock.ExpectQuery("SELECT (.+) FROM state WHERE possum=").WillReturnRows(fRows)
	mock.ExpectQuery("SELECT (.+) FROM state WHERE possum=").WillReturnRows(jRows)
	return db, err
}

func mockErrDBConn(driverName string, connectionString string) (*sql.DB, error) {
	db, _, err := sqlmock.New()
	err = fmt.Errorf("An error has occurred: %s", "Conn String Fetch Error")
	return db, err
}

func mockCreateController(db *sql.DB) *webs.Controller {
	return &webs.Controller{
		DB: &sql.DB{},
	}
}

var _ = Describe("Server", func() {
	Describe("#CreateServer", func() {
		var vcapServicesJSON string
		BeforeEach(func() {
			vcapServicesJSON = `{
  "user-provided": [
   {
    "credentials": {
      "username":"test_user",
      "password":"test_password",
      "host":"test_host",
      "port":"test_port",
      "database":"test_database"
    },
    "label": "user-provided",
    "name": "possum-db",
    "syslog_drain_url": "",
    "tags": []
   },
   {
    "credentials": {
      "passel": [
        "mother",
        "father",
        "joey"
      ]
    },
    "label": "user-provided",
    "name": "possum",
    "syslog_drain_url": "",
    "tags": []
   }
]
 }`
		})

		JustBeforeEach(func() {
			os.Setenv("VCAP_SERVICES", vcapServicesJSON)
			os.Setenv("VCAP_APPLICATION", "{}")
		})

		AfterEach(func() {
			os.Unsetenv("VCAP_APPLICATION")
			os.Unsetenv("VCAP_SERVICES")
		})

		Context("When possum-db service is set", func() {
			Context("and fetching the connection string raises an error", func() {
				It("returns an error", func() {
					_, err := webs.CreateServer(mockErrDBConn, mockCreateController)
					Ω(err).Should(MatchError("An error has occurred: Conn String Fetch Error"))
				})
			})

			Context("and fetching the connection string does not raise an error", func() {
				Context("and SetupStateDB does not raise an error", func() {
					It("creates a Server object", func() {
						server, err := webs.CreateServer(mockDBConn, mockCreateController)
						Ω(err).Should(BeNil())
						Ω(server).To(BeAssignableToTypeOf(&webs.Server{}))
					})
				})
				Context("and SetupStateDB raises an error", func() {
					It("returns an error", func() {
						_, err := webs.CreateServer(mockFailedStateDBConn, mockCreateController)
						Ω(err).Should(MatchError("An error has occurred: Database Create Error"))
					})
				})
			})

			Context("When possum-db service is not set", func() {
				BeforeEach(func() {
					vcapServicesJSON = `{
  "user-provided": [
   {
    "credenti
      "password":"test_password",
      "host":"test_host",
      "port":"test_port",
      "database":"test_database"
    },
    "label": "user-provided",
    "name": "possum-db",
    "syslog_drain_url": "",
    "tags": []
   }
  ]
 }`
				})

				It("returns an error", func() {
					_, err := webs.CreateServer(mockDBConn, mockCreateController)
					Ω(err).Should(MatchError("invalid character '\\n' in string literal"))
				})
			})
		})
	})
})

var _ = Describe("Controller", func() {
	var (
		db   *sql.DB
		mock sqlmock.Sqlmock
		err  error
	)

	BeforeEach(func() {
		db, mock, err = sqlmock.New()
		if err != nil {
			fmt.Printf("\nan error '%s' was not expected when opening a stub database connection\n", err)
			os.Exit(1)
		}
	})

	AfterEach(func() {
		db.Close()
		os.Unsetenv("VCAP_APPLICATION")
		os.Unsetenv("VCAP_SERVICES")
	})

	Describe("#GetState", func() {
		var (
			controller   *webs.Controller
			req          *http.Request
			mockRecorder *httptest.ResponseRecorder
		)

		JustBeforeEach(func() {
			req, _ = http.NewRequest("GET", "http://example.com/v1/state", nil)
			Router(controller).ServeHTTP(mockRecorder, req)
		})

		BeforeEach(func() {
			controller = webs.CreateController(db)
			mockRecorder = httptest.NewRecorder()
		})

		Context("when getting application uris raises an error", func() {
			It("returns an error 500", func() {
				Ω(mockRecorder.Code).Should(Equal(500))
				Ω(mockRecorder.Body.String()).Should(Equal(`{"error": "unexpected end of JSON input"}`))
			})
		})

		Context("when getting application uris does not error", func() {
			Context("and application uris is empty", func() {
				BeforeEach(func() {
					vcapApplicationJSON := `{
  "application_uris": []
}`
					os.Setenv("VCAP_APPLICATION", vcapApplicationJSON)
					os.Setenv("VCAP_SERVICES", "{}")
				})

				It("returns an error", func() {
					Ω(mockRecorder.Code).Should(Equal(410))
					Ω(mockRecorder.Body.String()).Should(Equal(`{"error": "No uris were configured"}`))
				})
			})

			Context("and application uris is not empty", func() {
				Context("and get passel raises an error", func() {
					BeforeEach(func() {
						vcapApplicationJSON := `{
  "application_uris": [
    "possum.example1.domain.com",
    "possum.example2.domain.com"
  ]
}`
						os.Setenv("VCAP_APPLICATION", vcapApplicationJSON)
						os.Setenv("VCAP_SERVICES", "{}")
					})

					It("returns an error 500", func() {
						Ω(mockRecorder.Code).Should(Equal(500))
						Ω(mockRecorder.Body.String()).Should(Equal(`{"error": "no service with name possum"}`))
					})
				})

				Context("and get passel does not raise an error", func() {
					Context("and passel has no members", func() {
						BeforeEach(func() {
							vcapApplicationJSON := `{
  "application_uris": [
    "possum.example1.domain.com",
    "possum.example2.domain.com"
  ]
}`
							vcapServicesJSON := `{
"user-provided": [
 {
  "credentials": {
    "passel": []
  },
  "label": "user-provided",
  "name": "possum",
  "syslog_drain_url": "",
  "tags": []
 }
]
}`
							os.Setenv("VCAP_APPLICATION", vcapApplicationJSON)
							os.Setenv("VCAP_SERVICES", vcapServicesJSON)
						})

						It("returns an error", func() {
							Ω(mockRecorder.Code).Should(Equal(410))
							Ω(mockRecorder.Body.String()).Should(Equal(`{"error": "Passel had 0 members"}`))
						})
					})

					Context("and passel has members", func() {
						Context("and there is no matching possum and application_uri", func() {
							BeforeEach(func() {
								vcapApplicationJSON := `{
  "application_uris": [
    "possum.example1.domain.com",
    "possum.example2.domain.com"
  ]
}`
								vcapServicesJSON := `{
"user-provided": [
 {
  "credentials": {
    "passel": [
      "mother",
      "father",
      "joey"
    ]
  },
  "label": "user-provided",
  "name": "possum",
  "syslog_drain_url": "",
  "tags": []
 }
]
}`
								os.Setenv("VCAP_APPLICATION", vcapApplicationJSON)
								os.Setenv("VCAP_SERVICES", vcapServicesJSON)
							})

							It("returns an error", func() {
								Ω(mockRecorder.Code).Should(Equal(410))
								Ω(mockRecorder.Body.String()).Should(Equal(`{"error": "Could not match any possum in db"}`))
							})

							Context("and there is a matching possum and application_uri", func() {
								BeforeEach(func() {
									vcapApplicationJSON := `{
  "application_uris": [
    "possum.example1.domain.com",
    "possum.example2.domain.com"
  ]
}`
									vcapServicesJSON := `{
"user-provided": [
 {
  "credentials": {
    "passel": [
      "https://possum.example1.domain.com",
      "father",
      "joey"
    ]
  },
  "label": "user-provided",
  "name": "possum",
  "syslog_drain_url": "",
  "tags": []
 }
]
}`
									os.Setenv("VCAP_APPLICATION", vcapApplicationJSON)
									os.Setenv("VCAP_SERVICES", vcapServicesJSON)
								})

								Context("and state cannot be found in db", func() {
									BeforeEach(func() {
										rows := sqlmock.NewRows([]string{"possum", "state"})

										mock.ExpectQuery("^SELECT (.+) FROM state WHERE possum=").WillReturnRows(rows)
									})

									It("returns an error", func() {
										Ω(mockRecorder.Code).Should(Equal(500))
										Ω(mockRecorder.Body.String()).Should(Equal(`{"error": "Could not find possum https://possum.example1.domain.com in db"}`))
									})
								})

								Context("and state can be found in db", func() {
									BeforeEach(func() {
										rows := sqlmock.NewRows([]string{"possum", "state"}).
											AddRow("possum.example1.domain.com", "alive")

										mock.ExpectQuery("^SELECT (.+) FROM state WHERE possum=").WillReturnRows(rows)
									})

									It("returns the state", func() {
										Ω(mockRecorder.Code).Should(Equal(200))
										Ω(mockRecorder.Body.String()).Should(Equal(`{"state": "alive"}`))
										Ω(mockRecorder.Header().Get("Content-Type")).Should(Equal("application/json"))
										Ω(mockRecorder.Header().Get("Access-Control-Allow-Origin")).Should(Equal("*"))
									})
								})
							})
						})
					})
				})
			})
		})
	})

	Describe("#GetPasselState", func() {
		var (
			controller   *webs.Controller
			req          *http.Request
			mockRecorder *httptest.ResponseRecorder
		)

		JustBeforeEach(func() {
			req, _ = http.NewRequest("GET", "http://example.com/v1/passel_state", nil)
			Router(controller).ServeHTTP(mockRecorder, req)
		})

		BeforeEach(func() {
			controller = webs.CreateController(db)
			mockRecorder = httptest.NewRecorder()
		})

		Context("and get passel raises an error", func() {
			BeforeEach(func() {
				os.Setenv("VCAP_APPLICATION", "{}")
				os.Setenv("VCAP_SERVICES", "{}")
			})

			It("returns an error 500", func() {
				Ω(mockRecorder.Code).Should(Equal(500))
				Ω(mockRecorder.Body.String()).Should(Equal(`{"error": "no service with name possum"}`))
			})
		})

		Context("and get passel does not raise an error", func() {
			Context("and passel has no members", func() {
				BeforeEach(func() {
					vcapServicesJSON := `{
"user-provided": [
 {
  "credentials": {
    "passel": []
  },
  "label": "user-provided",
  "name": "possum",
  "syslog_drain_url": "",
  "tags": []
 }
]
}`
					os.Setenv("VCAP_APPLICATION", "{}")
					os.Setenv("VCAP_SERVICES", vcapServicesJSON)
				})

				It("returns an error", func() {
					Ω(mockRecorder.Code).Should(Equal(500))
					Ω(mockRecorder.Body.String()).Should(Equal(`{"error": "Passel had 0 members"}`))
				})
			})

			Context("and passel has members", func() {
				BeforeEach(func() {
					vcapServicesJSON := `{
"user-provided": [
 {
  "credentials": {
    "passel": [
      "mother",
      "father",
      "joey"
    ]
  },
  "label": "user-provided",
  "name": "possum",
  "syslog_drain_url": "",
  "tags": []
 }
]
}`
					os.Setenv("VCAP_APPLICATION", "{}")
					os.Setenv("VCAP_SERVICES", vcapServicesJSON)
				})

				Context("and getting the states from db raises an error", func() {
					BeforeEach(func() {
						mRows := sqlmock.NewRows([]string{"possum", "state"})

						mock.ExpectQuery("^SELECT (.+) FROM state WHERE possum=").WithArgs("mother").WillReturnRows(mRows)
					})

					It("returns an error", func() {
						Ω(mockRecorder.Code).Should(Equal(500))
						Ω(mockRecorder.Body.String()).Should(Equal(`{"error": "Could not find possum mother in db"}`))
					})
				})

				Context("and the states can be fetched from the db", func() {
					BeforeEach(func() {
						mRows := sqlmock.NewRows([]string{"possum", "state"}).
							AddRow("mother", "alive")

						fRows := sqlmock.NewRows([]string{"possum", "state"}).
							AddRow("father", "alive")

						jRows := sqlmock.NewRows([]string{"possum", "state"}).
							AddRow("joey", "dead")

						mock.ExpectQuery("^SELECT (.+) FROM state WHERE possum=").WithArgs("mother").WillReturnRows(mRows)
						mock.ExpectQuery("^SELECT (.+) FROM state WHERE possum=").WithArgs("father").WillReturnRows(fRows)
						mock.ExpectQuery("^SELECT (.+) FROM state WHERE possum=").WithArgs("joey").WillReturnRows(jRows)
					})

					It("returns the state", func() {
						Ω(mockRecorder.Code).Should(Equal(200))
						Ω(mockRecorder.Body.String()).Should(Equal(`{"possum_states": {"father":"alive","joey":"dead","mother":"alive"}}`))
						Ω(mockRecorder.Header().Get("Content-Type")).Should(Equal("application/json"))
						Ω(mockRecorder.Header().Get("Access-Control-Allow-Origin")).Should(Equal("*"))
					})
				})
			})
		})
	})

	Describe("#GetPasselStateConsistency", func() {
		var (
			controller   *webs.Controller
			req          *http.Request
			mockRecorder *httptest.ResponseRecorder
		)

		JustBeforeEach(func() {
			req, _ = http.NewRequest("GET", "http://example.com/v1/passel_state_consistency", nil)
			Router(controller).ServeHTTP(mockRecorder, req)
		})

		BeforeEach(func() {
			controller = webs.CreateController(db)
			mockRecorder = httptest.NewRecorder()
		})

		Context("and get passel raises an error", func() {
			BeforeEach(func() {
				os.Setenv("VCAP_APPLICATION", "{}")
				os.Setenv("VCAP_SERVICES", "{}")
			})

			It("returns an error 500", func() {
				Ω(mockRecorder.Code).Should(Equal(500))
				Ω(mockRecorder.Body.String()).Should(Equal(`{"error": "no service with name possum"}`))
			})
		})

		Context("and get passel does not raise an error", func() {
			Context("and passel has no members", func() {
				BeforeEach(func() {
					vcapServicesJSON := `{
"user-provided": [
 {
  "credentials": {
    "passel": []
  },
  "label": "user-provided",
  "name": "possum",
  "syslog_drain_url": "",
  "tags": []
 }
]
}`
					os.Setenv("VCAP_APPLICATION", "{}")
					os.Setenv("VCAP_SERVICES", vcapServicesJSON)
				})

				It("returns an error", func() {
					Ω(mockRecorder.Code).Should(Equal(500))
					Ω(mockRecorder.Body.String()).Should(Equal(`{"error": "Passel had 0 members"}`))
				})
			})

			Context("and the passel has members", func() {
				Context("and getting passel states for a possum returns an error", func() {
					Context("due to passel state returning error json", func() {
						BeforeEach(func() {
							fakeServer1 = setup(MockRoute{"GET", "/v1/passel_state", `{"error": "I am an error"}`, "", 0})
							fakeServer2 = setup(MockRoute{"GET", "/v1/passel_state", `{}`, "", 0})
							vcapServicesJSON := fmt.Sprintf(`{
"user-provided": [
 {
  "credentials": {
    "passel": [
      "%s",
      "%s"
    ]
  },
  "label": "user-provided",
  "name": "possum",
  "syslog_drain_url": "",
  "tags": []
 }
]
}`, fakeServer1.URL, fakeServer2.URL)
							os.Setenv("VCAP_APPLICATION", "{}")
							os.Setenv("VCAP_SERVICES", vcapServicesJSON)
						})

						AfterEach(func() {
							teardown(fakeServer1)
							teardown(fakeServer2)
						})

						It("returns an error", func() {
							Ω(mockRecorder.Code).Should(Equal(500))
							Ω(mockRecorder.Body.String()).Should(Equal(`{"error": "I am an error"}`))
						})
					})
				})

				Context("due to passel state json being invalid", func() {
					BeforeEach(func() {
						fakeServer1 = setup(MockRoute{"GET", "/v1/passel_state", `{"error": "I am an err`, "", 0})
						fakeServer2 = setup(MockRoute{"GET", "/v1/passel_state", `{}`, "", 0})
						vcapServicesJSON := fmt.Sprintf(`{
"user-provided": [
 {
  "credentials": {
    "passel": [
      "%s",
      "%s"
    ]
  },
  "label": "user-provided",
  "name": "possum",
  "syslog_drain_url": "",
  "tags": []
 }
]
}`, fakeServer1.URL, fakeServer2.URL)
						os.Setenv("VCAP_APPLICATION", "{}")
						os.Setenv("VCAP_SERVICES", vcapServicesJSON)
					})

					AfterEach(func() {
						teardown(fakeServer1)
						teardown(fakeServer2)
					})

					It("returns an error", func() {
						Ω(mockRecorder.Code).Should(Equal(500))
						Ω(mockRecorder.Body.String()).Should(Equal(`{"error": "unexpected end of JSON input"}`))
					})
				})

				Context("due to an http request error", func() {
					BeforeEach(func() {
						vcapServicesJSON := fmt.Sprintf(`{
"user-provided": [
 {
  "credentials": {
    "passel": [
      "joey"
    ]
  },
  "label": "user-provided",
  "name": "possum",
  "syslog_drain_url": "",
  "tags": []
 }
]
}`)
						os.Setenv("VCAP_APPLICATION", "{}")
						os.Setenv("VCAP_SERVICES", vcapServicesJSON)
					})

					It("returns an error", func() {
						Ω(mockRecorder.Code).Should(Equal(500))
						Ω(mockRecorder.Body.String()).Should(Equal(`{"error": "Get joey/v1/passel_state: unsupported protocol scheme """}`))
					})
				})
			})

			Context("and getting passel states can be fetched from all possums", func() {
				Context("and state is inconsistent", func() {
					BeforeEach(func() {
						fakeServer1 = setup(MockRoute{"GET", "/v1/passel_state", `{"possum_states": {"father":"alive","joey":"dead","mother":"alive"}}`, "", 0})
						fakeServer2 = setup(MockRoute{"GET", "/v1/passel_state", `{"possum_states": {"father":"dead","joey":"dead","mother":"alive"}}`, "", 0})
						vcapServicesJSON := fmt.Sprintf(`{
"user-provided": [
 {
  "credentials": {
    "passel": [
      "%s",
      "%s"
    ]
  },
  "label": "user-provided",
  "name": "possum",
  "syslog_drain_url": "",
  "tags": []
 }
]
}`, fakeServer1.URL, fakeServer2.URL)
						os.Setenv("VCAP_APPLICATION", "{}")
						os.Setenv("VCAP_SERVICES", vcapServicesJSON)
					})

					AfterEach(func() {
						teardown(fakeServer1)
						teardown(fakeServer2)
					})

					It("returns an error and useful messages", func() {
						Ω(mockRecorder.Code).Should(Equal(500))
						Ω(mockRecorder.Body.String()).Should(Equal(`{"consistent": false, "error": "State was inconsistent", "passel_states": [{"father":"alive","joey":"dead","mother":"alive"},{"father":"dead","joey":"dead","mother":"alive"}]}`))
					})
				})

				Context("and state is consistent", func() {
					BeforeEach(func() {
						fakeServer1 = setup(MockRoute{"GET", "/v1/passel_state", `{"possum_states": {"father":"alive","joey":"dead","mother":"alive"}}`, "", 0})
						fakeServer2 = setup(MockRoute{"GET", "/v1/passel_state", `{"possum_states": {"mother":"alive","joey":"dead","father":"alive"}}`, "", 0})
						vcapServicesJSON := fmt.Sprintf(`{
"user-provided": [
 {
  "credentials": {
    "passel": [
      "%s",
      "%s"
    ]
  },
  "label": "user-provided",
  "name": "possum",
  "syslog_drain_url": "",
  "tags": []
 }
]
}`, fakeServer1.URL, fakeServer2.URL)
						os.Setenv("VCAP_APPLICATION", "{}")
						os.Setenv("VCAP_SERVICES", vcapServicesJSON)
					})

					AfterEach(func() {
						teardown(fakeServer1)
						teardown(fakeServer2)
					})

					It("returns consistent true", func() {
						Ω(mockRecorder.Code).Should(Equal(200))
						Ω(mockRecorder.Body.String()).Should(Equal(`{"consistent": true, "passel_states": [{"father":"alive","joey":"dead","mother":"alive"},{"father":"alive","joey":"dead","mother":"alive"}]}`))
						Ω(mockRecorder.Header().Get("Content-Type")).Should(Equal("application/json"))
						Ω(mockRecorder.Header().Get("Access-Control-Allow-Origin")).Should(Equal("*"))
					})
				})
			})
		})
	})

	Describe("#SetState", func() {
		var (
			controller   *webs.Controller
			req          *http.Request
			mockRecorder *httptest.ResponseRecorder
			requestBody  io.Reader
		)

		Context("when not authenticated", func() {
			Context("as a password was not configured", func() {
				JustBeforeEach(func() {
					req, _ = http.NewRequest("POST", "http://example.com/v1/state", requestBody)
					req.SetBasicAuth("admin", "admin")
					Router(controller).ServeHTTP(mockRecorder, req)
				})

				BeforeEach(func() {
					controller = webs.CreateController(db)
					mockRecorder = httptest.NewRecorder()
					os.Setenv("VCAP_APPLICATION", "{}")
					os.Setenv("VCAP_SERVICES", `{
"user-provided": [
 {
  "credentials": {
    "username": "admin"
  },
  "label": "user-provided",
  "name": "possum",
  "syslog_drain_url": "",
  "tags": []
 }
]
}`)
				})

				It("return an authentication error", func() {
					Ω(mockRecorder.Code).Should(Equal(401))
					Ω(mockRecorder.Body.String()).Should(Equal(`401 Unauthorized`))
				})
			})

			Context("as a username was not configured", func() {
				JustBeforeEach(func() {
					req, _ = http.NewRequest("POST", "http://example.com/v1/state", requestBody)
					req.SetBasicAuth("admin", "admin")
					Router(controller).ServeHTTP(mockRecorder, req)
				})

				BeforeEach(func() {
					controller = webs.CreateController(db)
					mockRecorder = httptest.NewRecorder()
					os.Setenv("VCAP_APPLICATION", "{}")
					os.Setenv("VCAP_SERVICES", `{
"user-provided": [
 {
  "credentials": {
    "password": "admin"
  },
  "label": "user-provided",
  "name": "possum",
  "syslog_drain_url": "",
  "tags": []
 }
]
}`)
				})

				It("return an authentication error", func() {
					Ω(mockRecorder.Code).Should(Equal(401))
					Ω(mockRecorder.Body.String()).Should(Equal(`401 Unauthorized`))
				})
			})

			Context("as a password not supplied", func() {
				JustBeforeEach(func() {
					req, _ = http.NewRequest("POST", "http://example.com/v1/state", requestBody)
					req.SetBasicAuth("admin", "")
					Router(controller).ServeHTTP(mockRecorder, req)
				})

				BeforeEach(func() {
					controller = webs.CreateController(db)
					mockRecorder = httptest.NewRecorder()
					os.Setenv("VCAP_APPLICATION", "{}")
					os.Setenv("VCAP_SERVICES", `{
"user-provided": [
 {
  "credentials": {
    "username": "admin",
    "password": "admin"
  },
  "label": "user-provided",
  "name": "possum",
  "syslog_drain_url": "",
  "tags": []
 }
]
}`)
				})

				It("return an authentication error", func() {
					Ω(mockRecorder.Code).Should(Equal(401))
					Ω(mockRecorder.Body.String()).Should(Equal(`401 Unauthorized`))
				})
			})

			Context("as a username not supplied", func() {
				JustBeforeEach(func() {
					req, _ = http.NewRequest("POST", "http://example.com/v1/state", requestBody)
					req.SetBasicAuth("", "admin")
					Router(controller).ServeHTTP(mockRecorder, req)
				})

				BeforeEach(func() {
					controller = webs.CreateController(db)
					mockRecorder = httptest.NewRecorder()
					os.Setenv("VCAP_APPLICATION", "{}")
					os.Setenv("VCAP_SERVICES", `{
"user-provided": [
 {
  "credentials": {
    "username": "admin",
    "password": "admin"
  },
  "label": "user-provided",
  "name": "possum",
  "syslog_drain_url": "",
  "tags": []
 }
]
}`)
				})

				It("return an authentication error", func() {
					Ω(mockRecorder.Code).Should(Equal(401))
					Ω(mockRecorder.Body.String()).Should(Equal(`401 Unauthorized`))
				})
			})

			Context("as a username supplied did not match", func() {
				JustBeforeEach(func() {
					req, _ = http.NewRequest("POST", "http://example.com/v1/state", requestBody)
					req.SetBasicAuth("not_admin", "admin")
					Router(controller).ServeHTTP(mockRecorder, req)
				})

				BeforeEach(func() {
					controller = webs.CreateController(db)
					mockRecorder = httptest.NewRecorder()
					os.Setenv("VCAP_APPLICATION", "{}")
					os.Setenv("VCAP_SERVICES", `{
"user-provided": [
 {
  "credentials": {
    "username": "admin",
    "password": "admin"
  },
  "label": "user-provided",
  "name": "possum",
  "syslog_drain_url": "",
  "tags": []
 }
]
}`)
				})

				It("return an authentication error", func() {
					Ω(mockRecorder.Code).Should(Equal(401))
					Ω(mockRecorder.Body.String()).Should(Equal(`401 Unauthorized`))
				})
			})

			Context("as a password supplied did not match", func() {
				JustBeforeEach(func() {
					req, _ = http.NewRequest("POST", "http://example.com/v1/state", requestBody)
					req.SetBasicAuth("admin", "not_admin")
					Router(controller).ServeHTTP(mockRecorder, req)
				})

				BeforeEach(func() {
					controller = webs.CreateController(db)
					mockRecorder = httptest.NewRecorder()
					os.Setenv("VCAP_APPLICATION", "{}")
					os.Setenv("VCAP_SERVICES", `{
"user-provided": [
 {
  "credentials": {
    "username": "admin",
    "password": "admin"
  },
  "label": "user-provided",
  "name": "possum",
  "syslog_drain_url": "",
  "tags": []
 }
]
}`)
				})

				It("return an authentication error", func() {
					Ω(mockRecorder.Code).Should(Equal(401))
					Ω(mockRecorder.Body.String()).Should(Equal(`401 Unauthorized`))
				})
			})
		})

		Context("when authenticated", func() {
			JustBeforeEach(func() {
				req, _ = http.NewRequest("POST", "http://example.com/v1/state", requestBody)
				req.SetBasicAuth("admin", "admin")
				Router(controller).ServeHTTP(mockRecorder, req)
			})

			BeforeEach(func() {
				controller = webs.CreateController(db)
				mockRecorder = httptest.NewRecorder()
			})

			Context("when getting application uris does not error", func() {
				Context("and application uris is empty", func() {
					BeforeEach(func() {
						vcapApplicationJSON := `{
  "application_uris": []
}`
						os.Setenv("VCAP_APPLICATION", vcapApplicationJSON)
						os.Setenv("VCAP_SERVICES", `{
"user-provided": [
 {
  "credentials": {
    "username": "admin",
    "password": "admin"
  },
  "label": "user-provided",
  "name": "possum",
  "syslog_drain_url": "",
  "tags": []
 }
]
}`)
					})

					It("returns an error", func() {
						Ω(mockRecorder.Code).Should(Equal(410))
						Ω(mockRecorder.Body.String()).Should(Equal(`{"error": "No uris were configured"}`))
					})
				})

				Context("and application uris is not empty", func() {
					Context("and get passel raises an error", func() {
						BeforeEach(func() {
							vcapApplicationJSON := `{
  "application_uris": [
    "possum.example1.domain.com",
    "possum.example2.domain.com"
  ]
}`
							vcapServicesJSON := `{
"user-provided": [
 {
  "credentials": {
    "username": "admin",
    "password": "admin",
    "passel": [
      1
    ]
  },
  "label": "user-provided",
  "name": "possum",
  "syslog_drain_url": "",
  "tags": []
 }
]
}`
							os.Setenv("VCAP_APPLICATION", vcapApplicationJSON)
							os.Setenv("VCAP_SERVICES", vcapServicesJSON)
						})

						It("returns an error 500", func() {
							Ω(mockRecorder.Code).Should(Equal(500))
							Ω(mockRecorder.Body.String()).Should(Equal(`{"error": "possum was not a string"}`))
						})
					})

					Context("and get passel does not raise an error", func() {
						Context("and passel has no members", func() {
							BeforeEach(func() {
								vcapApplicationJSON := `{
  "application_uris": [
    "possum.example1.domain.com",
    "possum.example2.domain.com"
  ]
}`
								vcapServicesJSON := `{
"user-provided": [
 {
  "credentials": {
    "username": "admin",
    "password": "admin",
    "passel": []
  },
  "label": "user-provided",
  "name": "possum",
  "syslog_drain_url": "",
  "tags": []
 }
]
}`
								os.Setenv("VCAP_APPLICATION", vcapApplicationJSON)
								os.Setenv("VCAP_SERVICES", vcapServicesJSON)
							})

							It("returns an error", func() {
								Ω(mockRecorder.Code).Should(Equal(410))
								Ω(mockRecorder.Body.String()).Should(Equal(`{"error": "Passel had 0 members"}`))
							})
						})

						Context("and passel has members", func() {
							Context("and there is no matching possum and application_uri", func() {
								BeforeEach(func() {
									vcapApplicationJSON := `{
  "application_uris": [
    "possum.example1.domain.com",
    "possum.example2.domain.com"
  ]
}`
									vcapServicesJSON := `{
"user-provided": [
 {
  "credentials": {
    "username": "admin",
    "password": "admin",
    "passel": [
      "mother",
      "father",
      "joey"
    ]
  },
  "label": "user-provided",
  "name": "possum",
  "syslog_drain_url": "",
  "tags": []
 }
]
}`
									os.Setenv("VCAP_APPLICATION", vcapApplicationJSON)
									os.Setenv("VCAP_SERVICES", vcapServicesJSON)
								})

								It("returns an error", func() {
									Ω(mockRecorder.Code).Should(Equal(410))
									Ω(mockRecorder.Body.String()).Should(Equal(`{"error": "Could not match any possum in db"}`))
								})

								Context("and there is a matching possum and application_uri", func() {
									Context("and passel state cannot be fetched", func() {
										BeforeEach(func() {
											fakeServer1 = setup(MockRoute{"GET", "/v1/passel_state", `{"error": "This is an error"}`, "", 0})
											requestBody = bytes.NewReader([]byte(`{}`))
											applicationURISplit := strings.Split(fakeServer1.URL, "http://")
											applicationURI := applicationURISplit[1]
											vcapApplicationJSON := fmt.Sprintf(`{
  "application_uris": [
    "%s"
  ]
}`, applicationURI)
											vcapServicesJSON := fmt.Sprintf(`{
"user-provided": [
 {
  "credentials": {
    "username": "admin",
    "password": "admin",
    "passel": [
      "%s"
    ]
  },
  "label": "user-provided",
  "name": "possum",
  "syslog_drain_url": "",
  "tags": []
 }
]
}`, fakeServer1.URL)
											os.Setenv("VCAP_APPLICATION", vcapApplicationJSON)
											os.Setenv("VCAP_SERVICES", vcapServicesJSON)
										})

										AfterEach(func() {
											teardown(fakeServer1)
										})

										It("returns an error", func() {
											Ω(mockRecorder.Code).Should(Equal(500))
											Ω(mockRecorder.Body.String()).Should(Equal(`{"error": "This is an error"}`))
										})
									})

									Context("and passel state can be fetched", func() {
										Context("and getting desired state raises an error", func() {
											BeforeEach(func() {
												fakeServer1 = setupMultiple([]MockRoute{
													{"GET", "/v1/passel_state", `{"possum_states": {"father":"alive","joey":"dead","mother":"alive"}}`, "", 0},
												})
												applicationURISplit := strings.Split(fakeServer1.URL, "http://")
												applicationURI := applicationURISplit[1]
												vcapApplicationJSON := fmt.Sprintf(`{
  "application_uris": [
    "%s"
  ]
}`, applicationURI)
												vcapServicesJSON := fmt.Sprintf(`{
"user-provided": [
 {
  "credentials": {
    "username": "admin",
    "password": "admin",
    "passel": [
      "%s"
    ]
  },
  "label": "user-provided",
  "name": "possum",
  "syslog_drain_url": "",
  "tags": []
 }
]
}`, fakeServer1.URL)
												os.Setenv("VCAP_APPLICATION", vcapApplicationJSON)
												os.Setenv("VCAP_SERVICES", vcapServicesJSON)
												requestBody = bytes.NewReader([]byte(`{[notjson}`))
											})

											AfterEach(func() {
												teardown(fakeServer1)
											})

											It("returns an error", func() {
												Ω(mockRecorder.Code).Should(Equal(500))
												Ω(mockRecorder.Body.String()).Should(Equal(`{"error": "invalid character '[' looking for beginning of object key string"}`))
											})
										})

										Context("and getting desired state is successful", func() {
											Context("and the desired state would kill all possums", func() {
												BeforeEach(func() {
													fakeServer1 = setupMultiple([]MockRoute{
														{"GET", "/v1/passel_state", `{"possum_states": {}}`, "", 0},
													})
													requestBody = bytes.NewReader([]byte(fmt.Sprintf(`{"%s":"dead"}`, fakeServer1.URL)))
													applicationURISplit := strings.Split(fakeServer1.URL, "http://")
													applicationURI := applicationURISplit[1]
													vcapApplicationJSON := fmt.Sprintf(`{
  "application_uris": [
    "%s"
  ]
}`, applicationURI)
													vcapServicesJSON := fmt.Sprintf(`{
"user-provided": [
 {
  "credentials": {
    "username": "admin",
    "password": "admin",
    "passel": [
      "%s"
    ]
  },
  "label": "user-provided",
  "name": "possum",
  "syslog_drain_url": "",
  "tags": []
 }
]
}`, fakeServer1.URL)
													os.Setenv("VCAP_APPLICATION", vcapApplicationJSON)
													os.Setenv("VCAP_SERVICES", vcapServicesJSON)
												})

												AfterEach(func() {
													teardown(fakeServer1)
												})

												It("returns an error", func() {
													Ω(mockRecorder.Code).Should(Equal(500))
													Ω(mockRecorder.Body.String()).Should(Equal(`{"error": "Would have killed all possums"}`))
												})
											})

											Context("and the desired possum is not in the passel", func() {
												BeforeEach(func() {
													requestBody = bytes.NewReader([]byte(`{"doesnotexist":"dead"}`))
													vcapApplicationJSON := fmt.Sprintf(`{
  "application_uris": [
    "%s"
  ]
}`, "joey.example.com")
													vcapServicesJSON := fmt.Sprintf(`{
"user-provided": [
 {
  "credentials": {
    "username": "admin",
    "password": "admin",
    "passel": [
      "%s"
    ]
  },
  "label": "user-provided",
  "name": "possum",
  "syslog_drain_url": "",
  "tags": []
 }
]
}`, "http://joey.example.com")
													os.Setenv("VCAP_APPLICATION", vcapApplicationJSON)
													os.Setenv("VCAP_SERVICES", vcapServicesJSON)
												})

												It("returns an error", func() {
													Ω(mockRecorder.Code).Should(Equal(500))
													Ω(mockRecorder.Body.String()).Should(Equal(`{"error": "Possum doesnotexist is not part of my passel"}`))
												})
											})

											Context("and all desired possums are in the passel", func() {
												Context("and the desired state would leave at least one possum alive", func() {
													Context("and the states cannot be written to the db", func() {
														BeforeEach(func() {
															fakeServer1 = setupMultiple([]MockRoute{
																{"GET", "/v1/passel_state", `{"possum_states": {"father":"alive","joey":"dead","mother":"alive"}}`, "", 0},
															})
															requestBody = bytes.NewReader([]byte(fmt.Sprintf(`{"%s":"dead"}`, fakeServer1.URL)))
															applicationURISplit := strings.Split(fakeServer1.URL, "http://")
															applicationURI := applicationURISplit[1]
															vcapApplicationJSON := fmt.Sprintf(`{
  "application_uris": [
    "%s"
  ]
}`, applicationURI)
															vcapServicesJSON := fmt.Sprintf(`{
"user-provided": [
 {
  "credentials": {
    "username": "admin",
    "password": "admin",
    "passel": [
      "%s"
    ]
  },
  "label": "user-provided",
  "name": "possum",
  "syslog_drain_url": "",
  "tags": []
 }
]
}`, fakeServer1.URL)
															os.Setenv("VCAP_APPLICATION", vcapApplicationJSON)
															os.Setenv("VCAP_SERVICES", vcapServicesJSON)
															mock.ExpectExec("UPDATE state.*").WillReturnError(fmt.Errorf("An error has occurred: %s", "UPDATE error"))

														})

														AfterEach(func() {
															teardown(fakeServer1)
														})

														It("returns an error", func() {
															Ω(mockRecorder.Code).Should(Equal(500))
															Ω(mockRecorder.Body.String()).Should(Equal(`{"error": "An error has occurred: UPDATE error"}`))
														})
													})

													Context("and the states can be written to the db", func() {
														BeforeEach(func() {
															mock.ExpectExec("UPDATE state.*").WillReturnResult(sqlmock.NewResult(1, 1))
															mock.ExpectExec("UPDATE state.*").WillReturnResult(sqlmock.NewResult(1, 1))
															mock.ExpectExec("UPDATE state.*").WillReturnResult(sqlmock.NewResult(1, 1))
														})

														Context("and getting the after write states raises an error", func() {
															BeforeEach(func() {
																fakeServer1 = setupMultiple([]MockRoute{
																	{"GET", "/v1/passel_state", `{"possum_states": {"father":"alive","joey":"dead","mother":"alive"}}`, "", 0},
																	{"GET", "/v1/passel_state", "", `{"possum_states": her":"dead","joey":"alive","mother":"dead"}}`, 1},
																})
																requestBody = bytes.NewReader([]byte(fmt.Sprintf(`{"%s":"dead"}`, fakeServer1.URL)))
																applicationURISplit := strings.Split(fakeServer1.URL, "http://")
																applicationURI := applicationURISplit[1]
																vcapApplicationJSON := fmt.Sprintf(`{
  "application_uris": [
    "%s"
  ]
}`, applicationURI)
																vcapServicesJSON := fmt.Sprintf(`{
"user-provided": [
 {
  "credentials": {
    "username": "admin",
    "password": "admin",
    "passel": [
      "%s"
    ]
  },
  "label": "user-provided",
  "name": "possum",
  "syslog_drain_url": "",
  "tags": []
 }
]
}`, fakeServer1.URL)
																os.Setenv("VCAP_APPLICATION", vcapApplicationJSON)
																os.Setenv("VCAP_SERVICES", vcapServicesJSON)
															})

															AfterEach(func() {
																teardown(fakeServer1)
															})

															It("returns an error", func() {
																Ω(mockRecorder.Code).Should(Equal(500))
																Ω(mockRecorder.Body.String()).Should(Equal(`{"error": "invalid character 'h' looking for beginning of value"}`))
															})
														})

														Context("and the after write states can be fetched", func() {
															Context("and the after state does not match the expected after state", func() {
																BeforeEach(func() {
																	fakeServer1 = setupMultiple([]MockRoute{
																		{"GET", "/v1/passel_state", `{"possum_states": {"father":"alive","joey":"dead","mother":"alive"}}`, "", 0},
																		{"GET", "/v1/passel_state", "", `{"possum_states": {"father":"alive","joey":"dead","mother":"alive"}}`, 1},
																	})
																	requestBody = bytes.NewReader([]byte(fmt.Sprintf(`{"%s":"dead"}`, fakeServer1.URL)))
																	applicationURISplit := strings.Split(fakeServer1.URL, "http://")
																	applicationURI := applicationURISplit[1]
																	vcapApplicationJSON := fmt.Sprintf(`{
  "application_uris": [
    "%s"
  ]
}`, applicationURI)
																	vcapServicesJSON := fmt.Sprintf(`{
"user-provided": [
 {
  "credentials": {
    "username": "admin",
    "password": "admin",
    "passel": [
      "%s"
    ]
  },
  "label": "user-provided",
  "name": "possum",
  "syslog_drain_url": "",
  "tags": []
 }
]
}`, fakeServer1.URL)
																	os.Setenv("VCAP_APPLICATION", vcapApplicationJSON)
																	os.Setenv("VCAP_SERVICES", vcapServicesJSON)
																})

																AfterEach(func() {
																	teardown(fakeServer1)
																})

																It("returns an error", func() {
																	Ω(mockRecorder.Code).Should(Equal(500))
																	Ω(mockRecorder.Body.String()).Should(Equal(fmt.Sprintf(`{"error": "State should have been: {"father":"alive","%s":"dead","joey":"dead","mother":"alive"} but was {"father":"alive","joey":"dead","mother":"alive"}"}`, fakeServer1.URL)))
																})
															})

															Context("and the after state matches the expected after state", func() {
																BeforeEach(func() {
																	fakeServer1 = setupMultiple([]MockRoute{
																		{"GET", "/v1/passel_state", `{"possum_states": {"father":"alive","joey":"dead","mother":"alive"}}`, "", 0},
																		{"GET", "/v1/passel_state", "", `{"possum_states": {"father":"dead","joey":"dead","mother":"alive"}}`, 1},
																	})
																	requestBody = bytes.NewReader([]byte(fmt.Sprintf(`{"%s":"dead"}`, "father")))
																	applicationURISplit := strings.Split(fakeServer1.URL, "http://")
																	applicationURI := applicationURISplit[1]
																	vcapApplicationJSON := fmt.Sprintf(`{
  "application_uris": [
    "%s"
  ]
}`, applicationURI)
																	vcapServicesJSON := fmt.Sprintf(`{
"user-provided": [
 {
  "credentials": {
    "username": "admin",
    "password": "admin",
    "passel": [
      "%s",
      "father"
    ]
  },
  "label": "user-provided",
  "name": "possum",
  "syslog_drain_url": "",
  "tags": []
 }
]
}`, fakeServer1.URL)
																	os.Setenv("VCAP_APPLICATION", vcapApplicationJSON)
																	os.Setenv("VCAP_SERVICES", vcapServicesJSON)
																})

																AfterEach(func() {
																	teardown(fakeServer1)
																})

																It("returns a http 202 and the configured states", func() {
																	Ω(mockRecorder.Code).Should(Equal(202))
																	Ω(mockRecorder.Body.String()).Should(Equal(`{"possum_states": {"father":"dead","joey":"dead","mother":"alive"}}`))
																})
															})
														})
													})
												})
											})
										})
									})
								})
							})
						})
					})
				})
			})
		})
	})

	Describe("#SetPasselState", func() {
		var (
			controller   *webs.Controller
			req          *http.Request
			mockRecorder *httptest.ResponseRecorder
			requestBody  io.Reader
		)
		Context("when not authenticated", func() {
			Context("as a password was not configured", func() {
				JustBeforeEach(func() {
					req, _ = http.NewRequest("POST", "http://example.com/v1/passel_state", requestBody)
					req.SetBasicAuth("admin", "admin")
					Router(controller).ServeHTTP(mockRecorder, req)
				})

				BeforeEach(func() {
					controller = webs.CreateController(db)
					mockRecorder = httptest.NewRecorder()
					os.Setenv("VCAP_APPLICATION", "{}")
					os.Setenv("VCAP_SERVICES", `{
"user-provided": [
 {
  "credentials": {
    "username": "admin"
  },
  "label": "user-provided",
  "name": "possum",
  "syslog_drain_url": "",
  "tags": []
 }
]
}`)
				})

				It("return an authentication error", func() {
					Ω(mockRecorder.Code).Should(Equal(401))
					Ω(mockRecorder.Body.String()).Should(Equal(`401 Unauthorized`))
				})
			})

			Context("as a username was not configured", func() {
				JustBeforeEach(func() {
					req, _ = http.NewRequest("POST", "http://example.com/v1/passel_state", requestBody)
					req.SetBasicAuth("admin", "admin")
					Router(controller).ServeHTTP(mockRecorder, req)
				})

				BeforeEach(func() {
					controller = webs.CreateController(db)
					mockRecorder = httptest.NewRecorder()
					os.Setenv("VCAP_APPLICATION", "{}")
					os.Setenv("VCAP_SERVICES", `{
"user-provided": [
 {
  "credentials": {
    "password": "admin"
  },
  "label": "user-provided",
  "name": "possum",
  "syslog_drain_url": "",
  "tags": []
 }
]
}`)
				})

				It("return an authentication error", func() {
					Ω(mockRecorder.Code).Should(Equal(401))
					Ω(mockRecorder.Body.String()).Should(Equal(`401 Unauthorized`))
				})
			})

			Context("as a password not supplied", func() {
				JustBeforeEach(func() {
					req, _ = http.NewRequest("POST", "http://example.com/v1/passel_state", requestBody)
					req.SetBasicAuth("admin", "")
					Router(controller).ServeHTTP(mockRecorder, req)
				})

				BeforeEach(func() {
					controller = webs.CreateController(db)
					mockRecorder = httptest.NewRecorder()
					os.Setenv("VCAP_APPLICATION", "{}")
					os.Setenv("VCAP_SERVICES", `{
"user-provided": [
 {
  "credentials": {
    "username": "admin",
    "password": "admin"
  },
  "label": "user-provided",
  "name": "possum",
  "syslog_drain_url": "",
  "tags": []
 }
]
}`)
				})

				It("return an authentication error", func() {
					Ω(mockRecorder.Code).Should(Equal(401))
					Ω(mockRecorder.Body.String()).Should(Equal(`401 Unauthorized`))
				})
			})

			Context("as a username not supplied", func() {
				JustBeforeEach(func() {
					req, _ = http.NewRequest("POST", "http://example.com/v1/passel_state", requestBody)
					req.SetBasicAuth("", "admin")
					Router(controller).ServeHTTP(mockRecorder, req)
				})

				BeforeEach(func() {
					controller = webs.CreateController(db)
					mockRecorder = httptest.NewRecorder()
					os.Setenv("VCAP_APPLICATION", "{}")
					os.Setenv("VCAP_SERVICES", `{
"user-provided": [
 {
  "credentials": {
    "username": "admin",
    "password": "admin"
  },
  "label": "user-provided",
  "name": "possum",
  "syslog_drain_url": "",
  "tags": []
 }
]
}`)
				})

				It("return an authentication error", func() {
					Ω(mockRecorder.Code).Should(Equal(401))
					Ω(mockRecorder.Body.String()).Should(Equal(`401 Unauthorized`))
				})
			})

			Context("as a username supplied did not match", func() {
				JustBeforeEach(func() {
					req, _ = http.NewRequest("POST", "http://example.com/v1/passel_state", requestBody)
					req.SetBasicAuth("not_admin", "admin")
					Router(controller).ServeHTTP(mockRecorder, req)
				})

				BeforeEach(func() {
					controller = webs.CreateController(db)
					mockRecorder = httptest.NewRecorder()
					os.Setenv("VCAP_APPLICATION", "{}")
					os.Setenv("VCAP_SERVICES", `{
"user-provided": [
 {
  "credentials": {
    "username": "admin",
    "password": "admin"
  },
  "label": "user-provided",
  "name": "possum",
  "syslog_drain_url": "",
  "tags": []
 }
]
}`)
				})

				It("return an authentication error", func() {
					Ω(mockRecorder.Code).Should(Equal(401))
					Ω(mockRecorder.Body.String()).Should(Equal(`401 Unauthorized`))
				})
			})

			Context("as a password supplied did not match", func() {
				JustBeforeEach(func() {
					req, _ = http.NewRequest("POST", "http://example.com/v1/passel_state", requestBody)
					req.SetBasicAuth("admin", "not_admin")
					Router(controller).ServeHTTP(mockRecorder, req)
				})

				BeforeEach(func() {
					controller = webs.CreateController(db)
					mockRecorder = httptest.NewRecorder()
					os.Setenv("VCAP_APPLICATION", "{}")
					os.Setenv("VCAP_SERVICES", `{
"user-provided": [
 {
  "credentials": {
    "username": "admin",
    "password": "admin"
  },
  "label": "user-provided",
  "name": "possum",
  "syslog_drain_url": "",
  "tags": []
 }
]
}`)
				})

				It("return an authentication error", func() {
					Ω(mockRecorder.Code).Should(Equal(401))
					Ω(mockRecorder.Body.String()).Should(Equal(`401 Unauthorized`))
				})
			})
		})

		Context("when authenticated", func() {
			JustBeforeEach(func() {
				req, _ = http.NewRequest("POST", "http://example.com/v1/passel_state", requestBody)
				req.SetBasicAuth("admin", "admin")
				Router(controller).ServeHTTP(mockRecorder, req)
			})

			BeforeEach(func() {
				controller = webs.CreateController(db)
				mockRecorder = httptest.NewRecorder()
			})

			Context("and get passel raises an error", func() {
				BeforeEach(func() {
					vcapApplicationJSON := `{
  "application_uris": [
    "possum.example1.domain.com",
    "possum.example2.domain.com"
  ]
}`

					vcapServicesJSON := `{
"user-provided": [
 {
  "credentials": {
    "username": "admin",
    "password": "admin",
    "passel": [
      1
    ]
  },
  "label": "user-provided",
  "name": "possum",
  "syslog_drain_url": "",
  "tags": []
 }
]
}`
					os.Setenv("VCAP_APPLICATION", vcapApplicationJSON)
					os.Setenv("VCAP_SERVICES", vcapServicesJSON)
				})

				It("returns an error 500", func() {
					Ω(mockRecorder.Code).Should(Equal(500))
					Ω(mockRecorder.Body.String()).Should(Equal(`{"error": "possum was not a string"}`))
				})
			})

			Context("and getting desired state raises an error", func() {
				BeforeEach(func() {
					requestBody = bytes.NewReader([]byte(`{[notjson}`))
					vcapServicesJSON := `{
"user-provided": [
 {
  "credentials": {
    "username": "admin",
    "password": "admin",
    "passel": [
      "http://joey"
    ]
  },
  "label": "user-provided",
  "name": "possum",
  "syslog_drain_url": "",
  "tags": []
 }
]
}`
					vcapApplicationJSON := `{
  "application_uris": [
    "joey"
  ]
}`
					os.Setenv("VCAP_APPLICATION", vcapApplicationJSON)
					os.Setenv("VCAP_SERVICES", vcapServicesJSON)
				})

				It("returns an error", func() {
					Ω(mockRecorder.Code).Should(Equal(500))
					Ω(mockRecorder.Body.String()).Should(Equal(`{"error": "invalid character '[' looking for beginning of object key string"}`))
				})
			})

			Context("and getting desired state is successful", func() {
				Context("and the passel has members", func() {
					Context("and the force flag is set", func() {
						Context("and the state would have killed all possums", func() {
							BeforeEach(func() {
								fakeServer1 = setup(MockRoute{"GET", "/v1/passel_state", `{"possum_states": {"father":"alive","joey":"dead","mother":"alive"}}`, "", 0})
								fakeServer2 = setup(MockRoute{"GET", "/v1/passel_state", `{"possum_states": {"father":"dead","joey":"dead","mother":"alive"}}`, "", 0})
								vcapServicesJSON := fmt.Sprintf(`{
"user-provided": [
 {
  "credentials": {
    "username": "admin",
    "password": "admin",
    "passel": [
      "%s",
      "%s"
    ]
  },
  "label": "user-provided",
  "name": "possum",
  "syslog_drain_url": "",
  "tags": []
 }
]
}`, fakeServer1.URL, fakeServer2.URL)
								applicationURISplit := strings.Split(fakeServer1.URL, "http://")
								applicationURI := applicationURISplit[1]
								vcapApplicationJSON := fmt.Sprintf(`{
  "application_uris": [
    "%s"
  ]
}`, applicationURI)
								os.Setenv("VCAP_APPLICATION", vcapApplicationJSON)
								os.Setenv("VCAP_SERVICES", vcapServicesJSON)
								requestBody = bytes.NewReader([]byte(`{"possum_states":{"mother": "dead", "father": "dead"}, "force": true}`))
							})

							AfterEach(func() {
								teardown(fakeServer1)
								teardown(fakeServer2)
							})

							It("returns an error and useful messages", func() {
								Ω(mockRecorder.Code).Should(Equal(500))
								Ω(mockRecorder.Body.String()).Should(Equal(`{"error": "Would have killed all possums"}`))
							})
						})

						Context("and the state would have left at least one possum alive", func() {
							Context("and set states returns an error", func() {
								Context("due to invalid json", func() {
									BeforeEach(func() {
										fakeServer1 = setupMultiple([]MockRoute{
											{"GET", "/v1/passel_state", `{"possum_states": {"father":"alive","joey":"dead","mother":"alive"}}`, "", 0},
											{"POST", "/v1/state", `{]`, "", 0},
										})
										fakeServer2 = setupMultiple([]MockRoute{
											{"GET", "/v1/passel_state", `{"possum_states": {"father":"alive","joey":"dead","mother":"alive"}}`, "", 0},
											{"POST", "/v1/state", `{]`, "", 0},
										})
										vcapServicesJSON := fmt.Sprintf(`{
"user-provided": [
 {
  "credentials": {
    "username": "admin",
    "password": "admin",
    "passel": [
      "%s",
      "%s"
    ]
  },
  "label": "user-provided",
  "name": "possum",
  "syslog_drain_url": "",
  "tags": []
 }
]
}`, fakeServer1.URL, fakeServer2.URL)
										applicationURISplit := strings.Split(fakeServer1.URL, "http://")
										applicationURI := applicationURISplit[1]
										vcapApplicationJSON := fmt.Sprintf(`{
  "application_uris": [
    "%s"
  ]
}`, applicationURI)
										os.Setenv("VCAP_APPLICATION", vcapApplicationJSON)
										os.Setenv("VCAP_SERVICES", vcapServicesJSON)
										requestBody = bytes.NewReader([]byte(`{"possum_states":{"joey": "alive", "mother": "alive"}}`))
									})

									AfterEach(func() {
										teardown(fakeServer1)
										teardown(fakeServer2)
									})

									It("returns an error", func() {
										Ω(mockRecorder.Code).Should(Equal(500))
										Ω(mockRecorder.Body.String()).Should(Equal(`{"error": "invalid character ']' looking for beginning of object key string"}`))
									})
								})

								Context("due to an error response", func() {
									BeforeEach(func() {
										fakeServer1 = setupMultiple([]MockRoute{
											{"GET", "/v1/passel_state", `{"possum_states": {"father":"alive","joey":"dead","mother":"alive"}}`, "", 0},
											{"POST", "/v1/state", `{"error": "this is an error"}`, "", 0},
										})
										fakeServer2 = setupMultiple([]MockRoute{
											{"GET", "/v1/passel_state", `{"possum_states": {"father":"dead","joey":"dead","mother":"alive"}}`, "", 0},
											{"POST", "/v1/state", `{"error": "this is an error"}`, "", 0},
										})
										vcapServicesJSON := fmt.Sprintf(`{
"user-provided": [
 {
  "credentials": {
    "username": "admin",
    "password": "admin",
    "passel": [
      "%s",
      "%s"
    ]
  },
  "label": "user-provided",
  "name": "possum",
  "syslog_drain_url": "",
  "tags": []
 }
]
}`, fakeServer1.URL, fakeServer2.URL)
										applicationURISplit := strings.Split(fakeServer1.URL, "http://")
										applicationURI := applicationURISplit[1]
										vcapApplicationJSON := fmt.Sprintf(`{
  "application_uris": [
    "%s"
  ]
}`, applicationURI)
										os.Setenv("VCAP_APPLICATION", vcapApplicationJSON)
										os.Setenv("VCAP_SERVICES", vcapServicesJSON)
										requestBody = bytes.NewReader([]byte(`{"possum_states":{"joey": "alive", "mother": "alive"}, "force": true}`))
									})

									AfterEach(func() {
										teardown(fakeServer1)
										teardown(fakeServer2)
									})

									It("returns an error", func() {
										Ω(mockRecorder.Code).Should(Equal(500))
										Ω(mockRecorder.Body.String()).Should(Equal(`{"error": "this is an error"}`))
									})
								})
							})

							Context("and set states is successful", func() {
								Context("and the after write passel states are inconsistent", func() {
									BeforeEach(func() {
										fakeServer1 = setupMultiple([]MockRoute{
											{"GET", "/v1/passel_state", `{"possum_states": {"father":"alive","joey":"dead","mother":"alive"}}`, "", 0},
											{"POST", "/v1/state", `{"possum_states": {"father":"alive", "joey": "alive", "mother": "alive"}}`, "", 0},
										})
										fakeServer2 = setupMultiple([]MockRoute{
											{"GET", "/v1/passel_state", `{"possum_states": {"father":"dead","joey":"dead","mother":"alive"}}`, "", 0},
											{"POST", "/v1/state", `{"possum_states": {"father":"dead", "joey": "alive", "mother": "alive"}}`, "", 0},
										})
										vcapServicesJSON := fmt.Sprintf(`{
"user-provided": [
 {
  "credentials": {
    "username": "admin",
    "password": "admin",
    "passel": [
      "%s",
      "%s"
    ]
  },
  "label": "user-provided",
  "name": "possum",
  "syslog_drain_url": "",
  "tags": []
 }
]
}`, fakeServer1.URL, fakeServer2.URL)
										applicationURISplit := strings.Split(fakeServer1.URL, "http://")
										applicationURI := applicationURISplit[1]
										vcapApplicationJSON := fmt.Sprintf(`{
  "application_uris": [
    "%s"
  ]
}`, applicationURI)
										os.Setenv("VCAP_APPLICATION", vcapApplicationJSON)
										os.Setenv("VCAP_SERVICES", vcapServicesJSON)
										requestBody = bytes.NewReader([]byte(`{"possum_states":{"joey": "alive", "mother": "alive"}, "force": true}`))
									})

									AfterEach(func() {
										teardown(fakeServer1)
										teardown(fakeServer2)
									})

									It("returns an error", func() {
										Ω(mockRecorder.Code).Should(Equal(500))
										Ω(mockRecorder.Body.String()).Should(Equal(`{"consistent": false, "error": "State was inconsistent after update", "passel_states": [{"father":"alive","joey":"alive","mother":"alive"},{"father":"dead","joey":"alive","mother":"alive"}]}`))
									})
								})

								Context("and the after write passel states are consistent", func() {
									BeforeEach(func() {
										fakeServer1 = setupMultiple([]MockRoute{
											{"GET", "/v1/passel_state", `{"possum_states": {"father":"alive","joey":"dead","mother":"alive"}}`, "", 0},
											{"POST", "/v1/state", `{"possum_states": {"father":"alive", "joey": "alive", "mother": "alive"}}`, "", 0},
										})
										fakeServer2 = setupMultiple([]MockRoute{
											{"GET", "/v1/passel_state", `{"possum_states": {"father":"dead","joey":"dead","mother":"alive"}}`, "", 0},
											{"POST", "/v1/state", `{"possum_states": {"father":"alive", "joey": "alive", "mother": "alive"}}`, "", 0},
										})
										vcapServicesJSON := fmt.Sprintf(`{
"user-provided": [
 {
  "credentials": {
    "username": "admin",
    "password": "admin",
    "passel": [
      "%s",
      "%s"
    ]
  },
  "label": "user-provided",
  "name": "possum",
  "syslog_drain_url": "",
  "tags": []
 }
]
}`, fakeServer1.URL, fakeServer2.URL)
										applicationURISplit := strings.Split(fakeServer1.URL, "http://")
										applicationURI := applicationURISplit[1]
										vcapApplicationJSON := fmt.Sprintf(`{
  "application_uris": [
    "%s"
  ]
}`, applicationURI)
										os.Setenv("VCAP_APPLICATION", vcapApplicationJSON)
										os.Setenv("VCAP_SERVICES", vcapServicesJSON)
										requestBody = bytes.NewReader([]byte(`{"possum_states":{"joey": "alive", "mother": "alive"}, "force": true}`))
									})

									AfterEach(func() {
										teardown(fakeServer1)
										teardown(fakeServer2)
									})

									It("returns a http 202 and consistent state", func() {
										Ω(mockRecorder.Code).Should(Equal(202))
										Ω(mockRecorder.Body.String()).Should(Equal(`{"consistent": true, "passel_states": [{"father":"alive","joey":"alive","mother":"alive"},{"father":"alive","joey":"alive","mother":"alive"}]}`))
									})
								})
							})
						})
					})

					Context("and the force flag is not set", func() {
						Context("and getting passel states for a possum returns an error", func() {
							Context("due to an http error", func() {
								BeforeEach(func() {
									fakeServer1 = setup(MockRoute{"GET", "/v1/passel_state", `{"error": "I am an error"}`, "", 0})
									fakeServer2 = setup(MockRoute{"GET", "/v1/passel_state", `{}`, "", 0})
									vcapServicesJSON := fmt.Sprintf(`{
"user-provided": [
 {
  "credentials": {
    "username": "admin",
    "password": "admin",
    "passel": [
      "%s",
      "%s"
    ]
  },
  "label": "user-provided",
  "name": "possum",
  "syslog_drain_url": "",
  "tags": []
 }
]
}`, fakeServer1.URL, fakeServer2.URL)
									applicationURISplit := strings.Split(fakeServer1.URL, "http://")
									applicationURI := applicationURISplit[1]
									vcapApplicationJSON := fmt.Sprintf(`{
  "application_uris": [
    "%s"
  ]
}`, applicationURI)
									os.Setenv("VCAP_APPLICATION", vcapApplicationJSON)
									os.Setenv("VCAP_SERVICES", vcapServicesJSON)
									requestBody = bytes.NewReader([]byte(`{"possum_states":{"joey": "alive", "mother": "alive"}}`))
									teardown(fakeServer1)
									teardown(fakeServer2)
								})

								It("returns an error", func() {
									Ω(mockRecorder.Code).Should(Equal(500))
									Ω(mockRecorder.Body.String()).Should(MatchRegexp(`{"error": "Get http://.+/v1/passel_state: dial tcp .+: getsockopt: connection refused"}`))
								})
							})

							Context("due to passel state returning error json", func() {
								BeforeEach(func() {
									fakeServer1 = setup(MockRoute{"GET", "/v1/passel_state", `{"error": "I am an error"}`, "", 0})
									fakeServer2 = setup(MockRoute{"GET", "/v1/passel_state", `{}`, "", 0})
									vcapServicesJSON := fmt.Sprintf(`{
"user-provided": [
 {
  "credentials": {
    "username": "admin",
    "password": "admin",
    "passel": [
      "%s",
      "%s"
    ]
  },
  "label": "user-provided",
  "name": "possum",
  "syslog_drain_url": "",
  "tags": []
 }
]
}`, fakeServer1.URL, fakeServer2.URL)
									applicationURISplit := strings.Split(fakeServer1.URL, "http://")
									applicationURI := applicationURISplit[1]
									vcapApplicationJSON := fmt.Sprintf(`{
  "application_uris": [
    "%s"
  ]
}`, applicationURI)
									os.Setenv("VCAP_APPLICATION", vcapApplicationJSON)
									os.Setenv("VCAP_SERVICES", vcapServicesJSON)
									requestBody = bytes.NewReader([]byte(`{"possum_states":{"joey": "alive", "mother": "alive"}}`))
								})

								AfterEach(func() {
									teardown(fakeServer1)
									teardown(fakeServer2)
								})

								It("returns an error", func() {
									Ω(mockRecorder.Code).Should(Equal(500))
									Ω(mockRecorder.Body.String()).Should(Equal(`{"error": "I am an error"}`))
								})
							})
						})

						Context("due to passel state json being invalid", func() {
							BeforeEach(func() {
								fakeServer1 = setup(MockRoute{"GET", "/v1/passel_state", `{"error": "I am an err`, "", 0})
								fakeServer2 = setup(MockRoute{"GET", "/v1/passel_state", `{}`, "", 0})
								vcapServicesJSON := fmt.Sprintf(`{
"user-provided": [
 {
  "credentials": {
    "username": "admin",
    "password": "admin",
    "passel": [
      "%s",
      "%s"
    ]
  },
  "label": "user-provided",
  "name": "possum",
  "syslog_drain_url": "",
  "tags": []
 }
]
}`, fakeServer1.URL, fakeServer2.URL)
								applicationURISplit := strings.Split(fakeServer1.URL, "http://")
								applicationURI := applicationURISplit[1]
								vcapApplicationJSON := fmt.Sprintf(`{
  "application_uris": [
    "%s"
  ]
}`, applicationURI)
								os.Setenv("VCAP_APPLICATION", vcapApplicationJSON)
								os.Setenv("VCAP_SERVICES", vcapServicesJSON)
								requestBody = bytes.NewReader([]byte(`{"possum_states":{"joey": "alive", "mother": "alive"}}`))
							})

							AfterEach(func() {
								teardown(fakeServer1)
								teardown(fakeServer2)
							})

							It("returns an error", func() {
								Ω(mockRecorder.Code).Should(Equal(500))
								Ω(mockRecorder.Body.String()).Should(Equal(`{"error": "unexpected end of JSON input"}`))
							})
						})

						Context("due to an http request error", func() {
							BeforeEach(func() {
								vcapServicesJSON := fmt.Sprintf(`{
"user-provided": [
 {
  "credentials": {
    "username": "admin",
    "password": "admin",
    "passel": [
      "joey"
    ]
  },
  "label": "user-provided",
  "name": "possum",
  "syslog_drain_url": "",
  "tags": []
 }
]
}`)
								applicationURISplit := strings.Split(fakeServer1.URL, "http://")
								applicationURI := applicationURISplit[1]
								vcapApplicationJSON := fmt.Sprintf(`{
  "application_uris": [
    "%s"
  ]
}`, applicationURI)
								os.Setenv("VCAP_APPLICATION", vcapApplicationJSON)
								os.Setenv("VCAP_SERVICES", vcapServicesJSON)
								requestBody = bytes.NewReader([]byte(`{"possum_states":{"joey": "alive", "mother": "alive"}}`))
							})

							It("returns an error", func() {
								Ω(mockRecorder.Code).Should(Equal(500))
								Ω(mockRecorder.Body.String()).Should(Equal(`{"error": "Get joey/v1/passel_state: unsupported protocol scheme """}`))
							})
						})
					})

					Context("and getting passel states can be fetched from all possums", func() {
						Context("and state is inconsistent", func() {
							BeforeEach(func() {
								fakeServer1 = setup(MockRoute{"GET", "/v1/passel_state", `{"possum_states": {"father":"alive","joey":"dead","mother":"alive"}}`, "", 0})
								fakeServer2 = setup(MockRoute{"GET", "/v1/passel_state", `{"possum_states": {"father":"dead","joey":"dead","mother":"alive"}}`, "", 0})
								vcapServicesJSON := fmt.Sprintf(`{
"user-provided": [
 {
  "credentials": {
    "username": "admin",
    "password": "admin",
    "passel": [
      "%s",
      "%s"
    ]
  },
  "label": "user-provided",
  "name": "possum",
  "syslog_drain_url": "",
  "tags": []
 }
]
}`, fakeServer1.URL, fakeServer2.URL)
								applicationURISplit := strings.Split(fakeServer1.URL, "http://")
								applicationURI := applicationURISplit[1]
								vcapApplicationJSON := fmt.Sprintf(`{
  "application_uris": [
    "%s"
  ]
}`, applicationURI)
								os.Setenv("VCAP_APPLICATION", vcapApplicationJSON)
								os.Setenv("VCAP_SERVICES", vcapServicesJSON)
								requestBody = bytes.NewReader([]byte(`{"possum_states":{"joey": "alive", "mother": "alive"}}`))
							})

							AfterEach(func() {
								teardown(fakeServer1)
								teardown(fakeServer2)
							})

							It("returns an error and useful messages", func() {
								Ω(mockRecorder.Code).Should(Equal(500))
								Ω(mockRecorder.Body.String()).Should(Equal(`{"consistent": false, "error": "State was inconsistent before update", "passel_states": [{"father":"alive","joey":"dead","mother":"alive"},{"father":"dead","joey":"dead","mother":"alive"}]}`))
							})
						})

						Context("and state is consistent", func() {
							Context("and the state would have killed all possums", func() {
								BeforeEach(func() {
									fakeServer1 = setup(MockRoute{"GET", "/v1/passel_state", `{"possum_states": {"father":"alive","joey":"dead","mother":"alive"}}`, "", 0})
									fakeServer2 = setup(MockRoute{"GET", "/v1/passel_state", `{"possum_states": {"father":"alive","joey":"dead","mother":"alive"}}`, "", 0})
									vcapServicesJSON := fmt.Sprintf(`{
"user-provided": [
 {
  "credentials": {
    "username": "admin",
    "password": "admin",
    "passel": [
      "%s",
      "%s"
    ]
  },
  "label": "user-provided",
  "name": "possum",
  "syslog_drain_url": "",
  "tags": []
 }
]
}`, fakeServer1.URL, fakeServer2.URL)
									applicationURISplit := strings.Split(fakeServer1.URL, "http://")
									applicationURI := applicationURISplit[1]
									vcapApplicationJSON := fmt.Sprintf(`{
  "application_uris": [
    "%s"
  ]
}`, applicationURI)
									os.Setenv("VCAP_APPLICATION", vcapApplicationJSON)
									os.Setenv("VCAP_SERVICES", vcapServicesJSON)
									requestBody = bytes.NewReader([]byte(`{"possum_states":{"mother": "dead", "father": "dead"}}`))
								})

								AfterEach(func() {
									teardown(fakeServer1)
									teardown(fakeServer2)
								})

								It("returns an error and useful messages", func() {
									Ω(mockRecorder.Code).Should(Equal(500))
									Ω(mockRecorder.Body.String()).Should(Equal(`{"error": "Would have killed all possums"}`))
								})
							})

							Context("and the state would have left at least one possum alive", func() {
								Context("and set states returns an error", func() {
									Context("due to invalid json", func() {
										BeforeEach(func() {
											fakeServer1 = setupMultiple([]MockRoute{
												{"GET", "/v1/passel_state", `{"possum_states": {"father":"alive","joey":"dead","mother":"alive"}}`, "", 0},
												{"POST", "/v1/state", `{]`, "", 0},
											})
											fakeServer2 = setupMultiple([]MockRoute{
												{"GET", "/v1/passel_state", `{"possum_states": {"father":"alive","joey":"dead","mother":"alive"}}`, "", 0},
												{"POST", "/v1/state", `{]`, "", 0},
											})
											vcapServicesJSON := fmt.Sprintf(`{
"user-provided": [
 {
  "credentials": {
    "username": "admin",
    "password": "admin",
    "passel": [
      "%s",
      "%s"
    ]
  },
  "label": "user-provided",
  "name": "possum",
  "syslog_drain_url": "",
  "tags": []
 }
]
}`, fakeServer1.URL, fakeServer2.URL)
											applicationURISplit := strings.Split(fakeServer1.URL, "http://")
											applicationURI := applicationURISplit[1]
											vcapApplicationJSON := fmt.Sprintf(`{
  "application_uris": [
    "%s"
  ]
}`, applicationURI)
											os.Setenv("VCAP_APPLICATION", vcapApplicationJSON)
											os.Setenv("VCAP_SERVICES", vcapServicesJSON)
											requestBody = bytes.NewReader([]byte(`{"possum_states":{"joey": "alive", "mother": "alive"}}`))
										})

										AfterEach(func() {
											teardown(fakeServer1)
											teardown(fakeServer2)
										})

										It("returns an error", func() {
											Ω(mockRecorder.Code).Should(Equal(500))
											Ω(mockRecorder.Body.String()).Should(Equal(`{"error": "invalid character ']' looking for beginning of object key string"}`))
										})
									})

									Context("due to an error response", func() {
										BeforeEach(func() {
											fakeServer1 = setupMultiple([]MockRoute{
												{"GET", "/v1/passel_state", `{"possum_states": {"father":"alive","joey":"dead","mother":"alive"}}`, "", 0},
												{"POST", "/v1/state", `{"error": "this is an error"}`, "", 0},
											})
											fakeServer2 = setupMultiple([]MockRoute{
												{"GET", "/v1/passel_state", `{"possum_states": {"father":"alive","joey":"dead","mother":"alive"}}`, "", 0},
												{"POST", "/v1/state", `{"error": "this is an error"}`, "", 0},
											})
											vcapServicesJSON := fmt.Sprintf(`{
"user-provided": [
 {
  "credentials": {
    "username": "admin",
    "password": "admin",
    "passel": [
      "%s",
      "%s"
    ]
  },
  "label": "user-provided",
  "name": "possum",
  "syslog_drain_url": "",
  "tags": []
 }
]
}`, fakeServer1.URL, fakeServer2.URL)
											applicationURISplit := strings.Split(fakeServer1.URL, "http://")
											applicationURI := applicationURISplit[1]
											vcapApplicationJSON := fmt.Sprintf(`{
  "application_uris": [
    "%s"
  ]
}`, applicationURI)
											os.Setenv("VCAP_APPLICATION", vcapApplicationJSON)
											os.Setenv("VCAP_SERVICES", vcapServicesJSON)
											requestBody = bytes.NewReader([]byte(`{"possum_states":{"joey": "alive", "mother": "alive"}}`))
										})

										AfterEach(func() {
											teardown(fakeServer1)
											teardown(fakeServer2)
										})

										It("returns an error", func() {
											Ω(mockRecorder.Code).Should(Equal(500))
											Ω(mockRecorder.Body.String()).Should(Equal(`{"error": "this is an error"}`))
										})
									})
								})

								Context("and set states is successful", func() {
									Context("and the after write passel states are inconsistent", func() {
										BeforeEach(func() {
											fakeServer1 = setupMultiple([]MockRoute{
												{"GET", "/v1/passel_state", `{"possum_states": {"father":"alive","joey":"dead","mother":"alive"}}`, "", 0},
												{"POST", "/v1/state", `{"possum_states": {"father":"alive", "joey": "alive", "mother": "alive"}}`, "", 0},
											})
											fakeServer2 = setupMultiple([]MockRoute{
												{"GET", "/v1/passel_state", `{"possum_states": {"father":"alive","joey":"dead","mother":"alive"}}`, "", 0},
												{"POST", "/v1/state", `{"possum_states": {"father":"dead", "joey": "alive", "mother": "alive"}}`, "", 0},
											})
											vcapServicesJSON := fmt.Sprintf(`{
"user-provided": [
 {
  "credentials": {
    "username": "admin",
    "password": "admin",
    "passel": [
      "%s",
      "%s"
    ]
  },
  "label": "user-provided",
  "name": "possum",
  "syslog_drain_url": "",
  "tags": []
 }
]
}`, fakeServer1.URL, fakeServer2.URL)
											applicationURISplit := strings.Split(fakeServer1.URL, "http://")
											applicationURI := applicationURISplit[1]
											vcapApplicationJSON := fmt.Sprintf(`{
  "application_uris": [
    "%s"
  ]
}`, applicationURI)
											os.Setenv("VCAP_APPLICATION", vcapApplicationJSON)
											os.Setenv("VCAP_SERVICES", vcapServicesJSON)
											requestBody = bytes.NewReader([]byte(`{"possum_states":{"joey": "alive", "mother": "alive"}}`))
										})

										AfterEach(func() {
											teardown(fakeServer1)
											teardown(fakeServer2)
										})

										It("returns an error", func() {
											Ω(mockRecorder.Code).Should(Equal(500))
											Ω(mockRecorder.Body.String()).Should(Equal(`{"consistent": false, "error": "State was inconsistent after update", "passel_states": [{"father":"alive","joey":"alive","mother":"alive"},{"father":"dead","joey":"alive","mother":"alive"}]}`))
										})
									})

									Context("and the after write passel states are consistent", func() {
										BeforeEach(func() {
											fakeServer1 = setupMultiple([]MockRoute{
												{"GET", "/v1/passel_state", `{"possum_states": {"father":"alive","joey":"dead","mother":"alive"}}`, "", 0},
												{"POST", "/v1/state", `{"possum_states": {"father":"alive", "joey": "alive", "mother": "alive"}}`, "", 0},
											})
											fakeServer2 = setupMultiple([]MockRoute{
												{"GET", "/v1/passel_state", `{"possum_states": {"father":"alive","joey":"dead","mother":"alive"}}`, "", 0},
												{"POST", "/v1/state", `{"possum_states": {"father":"alive", "joey": "alive", "mother": "alive"}}`, "", 0},
											})
											vcapServicesJSON := fmt.Sprintf(`{
"user-provided": [
 {
  "credentials": {
    "username": "admin",
    "password": "admin",
    "passel": [
      "%s",
      "%s"
    ]
  },
  "label": "user-provided",
  "name": "possum",
  "syslog_drain_url": "",
  "tags": []
 }
]
}`, fakeServer1.URL, fakeServer2.URL)
											applicationURISplit := strings.Split(fakeServer1.URL, "http://")
											applicationURI := applicationURISplit[1]
											vcapApplicationJSON := fmt.Sprintf(`{
  "application_uris": [
    "%s"
  ]
}`, applicationURI)
											os.Setenv("VCAP_APPLICATION", vcapApplicationJSON)
											os.Setenv("VCAP_SERVICES", vcapServicesJSON)
											requestBody = bytes.NewReader([]byte(`{"possum_states":{"joey": "alive", "mother": "alive"}}`))
										})

										AfterEach(func() {
											teardown(fakeServer1)
											teardown(fakeServer2)
										})

										It("returns a http 202 and consistent state", func() {
											Ω(mockRecorder.Code).Should(Equal(202))
											Ω(mockRecorder.Body.String()).Should(Equal(`{"consistent": true, "passel_states": [{"father":"alive","joey":"alive","mother":"alive"},{"father":"alive","joey":"alive","mother":"alive"}]}`))
										})
									})
								})
							})
						})
					})
				})
			})
		})
	})
})
