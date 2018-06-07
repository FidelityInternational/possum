package utils_test

import (
	"fmt"
	"os"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/FidelityInternational/possum/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("GetMyApplicationURIs", func() {
	var vcapApplicationJSON string
	JustBeforeEach(func() {
		os.Setenv("VCAP_APPLICATION", vcapApplicationJSON)
		os.Setenv("VCAP_SERVICES", "{}")
	})

	AfterEach(func() {
		os.Unsetenv("VCAP_APPLICATION")
		os.Unsetenv("VCAP_SERVICES")
	})

	Context("when there is more than one", func() {
		BeforeEach(func() {
			vcapApplicationJSON = `{
  "application_uris": [
    "possum.example1.domain.com",
    "possum.example2.domain.com"
  ]
}`
		})

		It("returns all values as a slice of strings", func() {
			uris, err := utils.GetMyApplicationURIs()
			Ω(err).Should(BeNil())
			Ω(uris).Should(HaveLen(2))
			Ω(uris).Should(ContainElement("possum.example1.domain.com"))
			Ω(uris).Should(ContainElement("possum.example2.domain.com"))
		})
	})

	Context("when there is only one", func() {
		BeforeEach(func() {
			vcapApplicationJSON = `{
  "application_uris": [
    "possum.example1.domain.com"
  ]
}`
		})

		It("returns all values as a slice of strings", func() {
			uris, err := utils.GetMyApplicationURIs()
			Ω(err).Should(BeNil())
			Ω(uris).Should(HaveLen(1))
			Ω(uris).Should(ContainElement("possum.example1.domain.com"))
		})
	})

	Context("when there are none", func() {
		BeforeEach(func() {
			vcapApplicationJSON = `{
  "application_uris": []
}`
		})
		It("returns an empty slice", func() {
			uris, err := utils.GetMyApplicationURIs()
			Ω(err).Should(BeNil())
			Ω(uris).Should(BeEmpty())
		})
	})

	Context("When unmarshalling raises an error", func() {
		BeforeEach(func() {
			vcapApplicationJSON = `{
  "name": "value",
  []
}`
		})

		It("Returns an error", func() {
			_, err := utils.GetMyApplicationURIs()
			Ω(err).Should(MatchError("invalid character '[' looking for beginning of object key string"))
		})
	})
})

var _ = Describe("GetDBConnectionDetails", func() {
	var vcapServicesJSON string

	JustBeforeEach(func() {
		os.Setenv("VCAP_SERVICES", vcapServicesJSON)
		os.Setenv("VCAP_APPLICATION", "{}")
	})

	AfterEach(func() {
		os.Unsetenv("VCAP_APPLICATION")
		os.Unsetenv("VCAP_SERVICES")
	})

	Context("when a possum-db service does not exist", func() {
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
  "name": "other-service",
  "syslog_drain_url": "",
  "tags": []
 }
]
}`
		})

		It("returns an error", func() {
			_, err := utils.GetDBConnectionDetails()
			Ω(err).Should(MatchError("no service with name possum-db"))
		})
	})

	Context("when a possum-db service exists", func() {
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
 }
]
}`
		})

		It("Returns the database connection string", func() {
			dbConnString, err := utils.GetDBConnectionDetails()
			Ω(err).Should(BeNil())
			Ω(dbConnString).Should(Equal("test_user:test_password@tcp(test_host:test_port)/test_database"))
		})

		Context("When unmarshaling a managed database connection", func() {
			BeforeEach(func() {
				vcapServicesJSON = `
     {
       "p-mysql": [
        {
         "credentials": {
          "hostname": "test_host",
          "jdbcUrl": "jdbc:mysql:/test_host:3306/test_database?user=test_user\u0026password=test_password",
          "name": "test_database",
          "password": "test_password",
          "port": 3306,
          "uri": "mysql://test_user:test_password@test_host:3306/test_database?reconnect=true",
          "username": "test_user"
         },
         "label": "p-mysql",
         "name": "possum-db",
         "plan": "512mb",
         "provider": null,
         "syslog_drain_url": null,
         "tags": [
          "mysql"
         ]
        }
       ]
     }`
			})

			It("returns the database connection string", func() {
				dbConnString, err := utils.GetDBConnectionDetails()
				Ω(err).Should(BeNil())
				Ω(dbConnString).Should(Equal("test_user:test_password@tcp(test_host:3306)/test_database"))
			})
		})

		Context("When unmarshalling raises an error", func() {
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

			It("Returns an error", func() {
				_, err := utils.GetDBConnectionDetails()
				Ω(err).Should(MatchError("invalid character '\\n' in string literal"))
			})
		})
	})
})

var _ = Describe("#SetupStateDB", func() {
	var vcapServicesJSON string

	JustBeforeEach(func() {
		os.Setenv("VCAP_SERVICES", vcapServicesJSON)
		os.Setenv("VCAP_APPLICATION", "{}")
	})

	AfterEach(func() {
		os.Unsetenv("VCAP_APPLICATION")
		os.Unsetenv("VCAP_SERVICES")
	})

	Context("When the table exists", func() {
		Context("when getting passel returns an error", func() {
			BeforeEach(func() {
				vcapServicesJSON = `{
  "user-provided": [
   {
    "credenti
      "key":"value"
    },
    "label": "user-provided",
    "name": "possum-db",
    "syslog_drain_url": "",
    "tags": []
   }
  ]
 }`
			})

			It("does nothing and returns an error", func() {
				db, mock, err := sqlmock.New()
				if err != nil {
					fmt.Printf("\nan error '%s' was not expected when opening a stub database connection\n", err)
					os.Exit(1)
				}
				defer db.Close()

				mock.ExpectExec("CREATE TABLE").WillReturnResult(sqlmock.NewResult(1, 1))
				Ω(utils.SetupStateDB(db)).Should(MatchError("invalid character '\\n' in string literal"))
			})
		})

		Context("when getting passel does not return an error", func() {
			BeforeEach(func() {
				vcapServicesJSON = `{
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
			})

			Context("when scanning the rows returns an error", func() {
				It("returns an error", func() {
					db, mock, err := sqlmock.New()
					if err != nil {
						fmt.Printf("\nan error '%s' was not expected when opening a stub database connection\n", err)
						os.Exit(1)
					}
					defer db.Close()
					mRows := sqlmock.NewRows([]string{"possum"}).
						AddRow("mother")

					mock.ExpectExec("CREATE TABLE").WillReturnResult(sqlmock.NewResult(1, 1))
					mock.ExpectQuery("SELECT (.+) FROM state WHERE possum=").WillReturnRows(mRows)
					Ω(utils.SetupStateDB(db)).Should(MatchError("sql: expected 1 destination arguments in Scan, not 2"))
				})
			})

			Context("when all possums are already in the db", func() {
				It("does not create database or insert rows", func() {
					db, mock, err := sqlmock.New()
					if err != nil {
						fmt.Printf("\nan error '%s' was not expected when opening a stub database connection\n", err)
						os.Exit(1)
					}
					defer db.Close()
					mRows := sqlmock.NewRows([]string{"possum", "state"}).
						AddRow("mother", "alive")
					fRows := sqlmock.NewRows([]string{"possum", "state"}).
						AddRow("father", "alive")
					jRows := sqlmock.NewRows([]string{"possum", "state"}).
						AddRow("joey", "alive")

					mock.ExpectExec("CREATE TABLE").WillReturnResult(sqlmock.NewResult(1, 1))
					mock.ExpectQuery("SELECT (.+) FROM state WHERE possum=").WillReturnRows(mRows)
					mock.ExpectQuery("SELECT (.+) FROM state WHERE possum=").WillReturnRows(fRows)
					mock.ExpectQuery("SELECT (.+) FROM state WHERE possum=").WillReturnRows(jRows)
					Ω(utils.SetupStateDB(db)).Should(BeNil())
				})
			})

			Context("when all possums are not in the db", func() {
				Context("and inserting the records returns an error", func() {
					It("returns an error", func() {
						db, mock, err := sqlmock.New()
						if err != nil {
							fmt.Printf("\nan error '%s' was not expected when opening a stub database connection\n", err)
							os.Exit(1)
						}
						defer db.Close()
						rows := sqlmock.NewRows([]string{"possum", "state"})

						mock.ExpectExec("CREATE TABLE").WillReturnResult(sqlmock.NewResult(1, 1))
						mock.ExpectQuery("SELECT (.+) FROM state WHERE possum=").WillReturnRows(rows)
						mock.ExpectExec("INSERT INTO state").WithArgs("mother").WillReturnResult(sqlmock.NewResult(1, 1))
						Ω(utils.SetupStateDB(db)).Should(MatchError("exec query 'INSERT INTO state VALUES (?, ?)', arguments do not match: expected 1, but got 2 arguments"))
					})
				})

				Context("and inserting the records does not return an error", func() {
					It("does not create database but does insert the possums with an alive state", func() {
						db, mock, err := sqlmock.New()
						if err != nil {
							fmt.Printf("\nan error '%s' was not expected when opening a stub database connection\n", err)
							os.Exit(1)
						}
						defer db.Close()
						rows := sqlmock.NewRows([]string{"possum", "state"})

						mock.ExpectExec("CREATE TABLE").WillReturnResult(sqlmock.NewResult(1, 1))
						mock.ExpectQuery("SELECT (.+) FROM state WHERE possum=").WillReturnRows(rows)
						mock.ExpectExec("INSERT INTO state").WithArgs("mother", "alive").WillReturnResult(sqlmock.NewResult(1, 1))
						mock.ExpectQuery("SELECT (.+) FROM state WHERE possum=").WillReturnRows(rows)
						mock.ExpectExec("INSERT INTO state").WithArgs("father", "alive").WillReturnResult(sqlmock.NewResult(1, 1))
						mock.ExpectQuery("SELECT (.+) FROM state WHERE possum=").WillReturnRows(rows)
						mock.ExpectExec("INSERT INTO state").WithArgs("joey", "alive").WillReturnResult(sqlmock.NewResult(1, 1))
						Ω(utils.SetupStateDB(db)).Should(BeNil())
					})
				})
			})
		})
	})

	Context("When the table does not exist", func() {
		BeforeEach(func() {
			vcapServicesJSON = `{
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
		})

		It("creates the table", func() {
			db, mock, err := sqlmock.New()
			if err != nil {
				fmt.Printf("\nan error '%s' was not expected when opening a stub database connection\n", err)
				os.Exit(1)
			}
			defer db.Close()
			mRows := sqlmock.NewRows([]string{"possum", "state"}).
				AddRow("mother", "alive")
			fRows := sqlmock.NewRows([]string{"possum", "state"}).
				AddRow("father", "alive")
			jRows := sqlmock.NewRows([]string{"possum", "state"}).
				AddRow("joey", "alive")

			mock.ExpectExec("CREATE TABLE").WillReturnResult(sqlmock.NewResult(1, 1))
			mock.ExpectQuery("SELECT (.+) FROM state WHERE possum=").WillReturnRows(mRows)
			mock.ExpectQuery("SELECT (.+) FROM state WHERE possum=").WillReturnRows(fRows)
			mock.ExpectQuery("SELECT (.+) FROM state WHERE possum=").WillReturnRows(jRows)
			mock.ExpectExec("CREATE TABLE").WillReturnResult(sqlmock.NewResult(1, 1))
			Ω(utils.SetupStateDB(db)).Should(BeNil())
		})
	})

	Context("When the create command returns an error", func() {
		It("Returns an error", func() {
			db, mock, err := sqlmock.New()
			if err != nil {
				fmt.Printf("\nan error '%s' was not expected when opening a stub database connection\n", err)
				os.Exit(1)
			}
			defer db.Close()

			mock.ExpectExec("CREATE TABLE").WillReturnError(fmt.Errorf("An error has occurred: %s", "Database Create Error"))
			err = utils.SetupStateDB(db)
			Ω(err).ShouldNot(BeNil())
			Ω(err.Error()).Should(Equal("An error has occurred: Database Create Error"))
		})
	})
})

var _ = Describe("GetPassel", func() {
	var vcapServicesJSON string

	JustBeforeEach(func() {
		os.Setenv("VCAP_SERVICES", vcapServicesJSON)
		os.Setenv("VCAP_APPLICATION", "{}")
	})

	AfterEach(func() {
		os.Unsetenv("VCAP_APPLICATION")
		os.Unsetenv("VCAP_SERVICES")
	})

	Context("when a possum service does not exist", func() {
		BeforeEach(func() {
			vcapServicesJSON = `{
"user-provided": [
 {
  "credentials": {
    "key":"value"
  },
  "label": "user-provided",
  "name": "other-service",
  "syslog_drain_url": "",
  "tags": []
 }
]
}`
		})

		It("returns an error", func() {
			_, err := utils.GetPassel()
			Ω(err).Should(MatchError("no service with name possum"))
		})
	})

	Context("when a possum service exists", func() {
		Context("and a passel was not a string", func() {
			BeforeEach(func() {
				vcapServicesJSON = `{
"user-provided": [
 {
  "credentials": {
    "passel": [
      "mother",
      "father",
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
			})

			It("returns an error", func() {
				_, err := utils.GetPassel()
				Ω(err).Should(MatchError("possum was not a string"))
			})
		})

		Context("and all passels are strings", func() {
			BeforeEach(func() {
				vcapServicesJSON = `{
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
			})

			It("Returns an array of possums", func() {
				passel, err := utils.GetPassel()
				Ω(err).Should(BeNil())
				Ω(passel).Should(HaveLen(3))
				Ω(passel).Should(ContainElement("mother"))
				Ω(passel).Should(ContainElement("father"))
				Ω(passel).Should(ContainElement("joey"))
			})
		})

		Context("When unmarshalling raises an error", func() {
			BeforeEach(func() {
				vcapServicesJSON = `{
  "user-provided": [
   {
    "credenti
      "key":"value"
    },
    "label": "user-provided",
    "name": "possum-db",
    "syslog_drain_url": "",
    "tags": []
   }
  ]
 }`
			})

			It("Returns an error", func() {
				_, err := utils.GetPassel()
				Ω(err).Should(MatchError("invalid character '\\n' in string literal"))
			})
		})
	})
})

var _ = Describe("#GetState", func() {
	Context("When the possum exists", func() {
		It("Returns the state", func() {
			db, mock, err := sqlmock.New()
			if err != nil {
				fmt.Printf("\nan error '%s' was not expected when opening a stub database connection\n", err)
				os.Exit(1)
			}
			defer db.Close()

			rows := sqlmock.NewRows([]string{"possum", "state"}).
				AddRow("joey", "alive")

			mock.ExpectQuery("^SELECT (.+) FROM state WHERE possum=").WillReturnRows(rows)

			state, err := utils.GetState(db, "joey")
			Ω(err).Should(BeNil())
			Ω(state).Should(Equal("alive"))
		})
	})

	Context("When the possum does not exist", func() {
		It("Returns an error", func() {
			db, mock, err := sqlmock.New()
			if err != nil {
				fmt.Printf("\nan error '%s' was not expected when opening a stub database connection\n", err)
				os.Exit(1)
			}
			defer db.Close()

			rows := sqlmock.NewRows([]string{"possum", "state"})
			mock.ExpectQuery("^SELECT (.+) FROM state WHERE possum=").WillReturnRows(rows)

			state, err := utils.GetState(db, "joey")
			Ω(err).Should(MatchError("Could not find possum joey in db"))
			Ω(state).Should(Equal(""))
		})
	})

	Context("when a row cannot be scanned", func() {
		It("returns an error", func() {
			db, mock, err := sqlmock.New()
			if err != nil {
				fmt.Printf("\nan error '%s' was not expected when opening a stub database connection\n", err)
				os.Exit(1)
			}
			defer db.Close()

			rows := sqlmock.NewRows([]string{"possum"}).
				AddRow("joey")

			mock.ExpectQuery("^SELECT (.+) FROM state WHERE possum=").WillReturnRows(rows)

			state, err := utils.GetState(db, "joey")
			Ω(err).Should(MatchError("sql: expected 1 destination arguments in Scan, not 2"))
			Ω(state).Should(Equal(""))
		})
	})
})

var _ = Describe("#GetPasselState", func() {
	Context("When passel is empty", func() {
		var passel = []string{}
		It("Returns an error", func() {
			db, _, err := sqlmock.New()
			if err != nil {
				fmt.Printf("\nan error '%s' was not expected when opening a stub database connection\n", err)
				os.Exit(1)
			}
			defer db.Close()

			passelState, err := utils.GetPasselState(db, passel)
			Ω(err).Should(MatchError("Passel had 0 members"))
			Ω(passelState).Should(BeNil())
		})
	})

	Context("When passel is not empty", func() {
		var passel = []string{"father", "mother", "joey"}
		Context("When all possums exists", func() {
			It("Returns the passel state", func() {
				db, mock, err := sqlmock.New()
				if err != nil {
					fmt.Printf("\nan error '%s' was not expected when opening a stub database connection\n", err)
					os.Exit(1)
				}
				defer db.Close()

				mRows := sqlmock.NewRows([]string{"possum", "state"}).
					AddRow("mother", "alive")
				fRows := sqlmock.NewRows([]string{"possum", "state"}).
					AddRow("father", "dead")
				jRows := sqlmock.NewRows([]string{"possum", "state"}).
					AddRow("joey", "alive")

				mock.ExpectQuery("SELECT (.+) FROM state WHERE possum=").WithArgs("father").WillReturnRows(fRows)
				mock.ExpectQuery("SELECT (.+) FROM state WHERE possum=").WithArgs("mother").WillReturnRows(mRows)
				mock.ExpectQuery("SELECT (.+) FROM state WHERE possum=").WithArgs("joey").WillReturnRows(jRows)

				passelState, err := utils.GetPasselState(db, passel)
				Ω(err).Should(BeNil())
				Ω(passelState).Should(HaveLen(3))
				Ω(passelState["father"]).Should(Equal("dead"))
				Ω(passelState["mother"]).Should(Equal("alive"))
				Ω(passelState["joey"]).Should(Equal("alive"))
			})
		})

		Context("When at least one possum does not exist", func() {
			It("Returns an error", func() {
				db, mock, err := sqlmock.New()
				if err != nil {
					fmt.Printf("\nan error '%s' was not expected when opening a stub database connection\n", err)
					os.Exit(1)
				}
				defer db.Close()

				mRows := sqlmock.NewRows([]string{"possum", "state"})
				fRows := sqlmock.NewRows([]string{"possum", "state"}).
					AddRow("father", "dead")
				jRows := sqlmock.NewRows([]string{"possum", "state"}).
					AddRow("joey", "alive")

				mock.ExpectQuery("SELECT (.+) FROM state WHERE possum=").WithArgs("father").WillReturnRows(fRows)
				mock.ExpectQuery("SELECT (.+) FROM state WHERE possum=").WithArgs("mother").WillReturnRows(mRows)
				mock.ExpectQuery("SELECT (.+) FROM state WHERE possum=").WithArgs("joey").WillReturnRows(jRows)
				state, err := utils.GetPasselState(db, passel)
				Ω(err).Should(MatchError("Could not find possum mother in db"))
				Ω(state).Should(BeNil())
			})
		})

		Context("when a row cannot be scanned", func() {
			It("returns an error", func() {
				db, mock, err := sqlmock.New()
				if err != nil {
					fmt.Printf("\nan error '%s' was not expected when opening a stub database connection\n", err)
					os.Exit(1)
				}
				defer db.Close()

				rows := sqlmock.NewRows([]string{"possum"}).
					AddRow("joey")

				mock.ExpectQuery("^SELECT (.+) FROM state WHERE possum=").WillReturnRows(rows)

				state, err := utils.GetPasselState(db, passel)
				Ω(err).Should(MatchError("sql: expected 1 destination arguments in Scan, not 2"))
				Ω(state).Should(BeNil())
			})
		})
	})
})

var _ = Describe("#WriteState", func() {
	Context("when the state is 'alive'", func() {
		It("Updates the possum state", func() {
			db, mock, err := sqlmock.New()
			if err != nil {
				fmt.Printf("\nan error '%s' was not expected when opening a stub database connection\n", err)
				os.Exit(1)
			}
			defer db.Close()

			mock.ExpectExec("UPDATE state.*").WithArgs("alive", "joey").WillReturnResult(sqlmock.NewResult(1, 1))
			Ω(utils.WriteState(db, "joey", "alive")).Should(BeNil())
		})
	})

	Context("when the state is 'dead'", func() {
		It("Updates the possum state", func() {
			db, mock, err := sqlmock.New()
			if err != nil {
				fmt.Printf("\nan error '%s' was not expected when opening a stub database connection\n", err)
				os.Exit(1)
			}
			defer db.Close()

			mock.ExpectExec("UPDATE state.*").WithArgs("dead", "joey").WillReturnResult(sqlmock.NewResult(1, 1))
			Ω(utils.WriteState(db, "joey", "dead")).Should(BeNil())
		})
	})

	Context("when the state is not 'alive' or 'dead", func() {
		It("returns an error", func() {
			db, _, err := sqlmock.New()
			if err != nil {
				fmt.Printf("\nan error '%s' was not expected when opening a stub database connection\n", err)
				os.Exit(1)
			}
			defer db.Close()

			err = utils.WriteState(db, "joey", "undead")
			Ω(err).Should(MatchError(`The state should have been "alive" or "dead" not "undead"`))
		})
	})

	Context("When the sql update command raises an error", func() {
		It("returns an error", func() {
			db, mock, err := sqlmock.New()
			if err != nil {
				fmt.Printf("\nan error '%s' was not expected when opening a stub database connection\n", err)
				os.Exit(1)
			}
			defer db.Close()

			mock.ExpectExec("UPDATE state.*").WithArgs("alive", "joey").WillReturnError(fmt.Errorf("An error has occurred: %s", "UPDATE error"))
			err = utils.WriteState(db, "joey", "alive")
			Ω(err).Should(MatchError("An error has occurred: UPDATE error"))
		})
	})
})

var _ = Describe("GetUsername", func() {
	var vcapServicesJSON string

	JustBeforeEach(func() {
		os.Setenv("VCAP_SERVICES", vcapServicesJSON)
		os.Setenv("VCAP_APPLICATION", "{}")
	})

	AfterEach(func() {
		os.Unsetenv("VCAP_APPLICATION")
		os.Unsetenv("VCAP_SERVICES")
	})

	Context("when a possum service does not exist", func() {
		BeforeEach(func() {
			vcapServicesJSON = `{
"user-provided": [
 {
  "credentials": {
    "key":"value"
  },
  "label": "user-provided",
  "name": "other-service",
  "syslog_drain_url": "",
  "tags": []
 }
]
}`
		})

		It("returns an error", func() {
			_, err := utils.GetUsername()
			Ω(err).Should(MatchError("no service with name possum"))
		})
	})

	Context("when a possum service exists", func() {
		Context("and a username is not present", func() {
			BeforeEach(func() {
				vcapServicesJSON = `{
"user-provided": [
 {
  "credentials": {
  },
  "label": "user-provided",
  "name": "possum",
  "syslog_drain_url": "",
  "tags": []
 }
]
}`
			})

			It("returns an empty string", func() {
				username, err := utils.GetUsername()
				Ω(err).Should(BeNil())
				Ω(username).Should(BeEmpty())
			})
		})

		Context("and a username is present", func() {
			BeforeEach(func() {
				vcapServicesJSON = `{
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
}`
			})

			It("Returns the configured username", func() {
				username, err := utils.GetUsername()
				Ω(err).Should(BeNil())
				Ω(username).Should(Equal("admin"))
			})
		})

		Context("When unmarshalling raises an error", func() {
			BeforeEach(func() {
				vcapServicesJSON = `{
  "user-provided": [
   {
    "credenti
      "key":"value"
    },
    "label": "user-provided",
    "name": "possum-db",
    "syslog_drain_url": "",
    "tags": []
   }
  ]
 }`
			})

			It("Returns an error", func() {
				_, err := utils.GetUsername()
				Ω(err).Should(MatchError("invalid character '\\n' in string literal"))
			})
		})
	})
})

var _ = Describe("GetPassword", func() {
	var vcapServicesJSON string

	JustBeforeEach(func() {
		os.Setenv("VCAP_SERVICES", vcapServicesJSON)
		os.Setenv("VCAP_APPLICATION", "{}")
	})

	AfterEach(func() {
		os.Unsetenv("VCAP_APPLICATION")
		os.Unsetenv("VCAP_SERVICES")
	})

	Context("when a possum service does not exist", func() {
		BeforeEach(func() {
			vcapServicesJSON = `{
"user-provided": [
 {
  "credentials": {
    "key":"value"
  },
  "label": "user-provided",
  "name": "other-service",
  "syslog_drain_url": "",
  "tags": []
 }
]
}`
		})

		It("returns an error", func() {
			_, err := utils.GetPassword()
			Ω(err).Should(MatchError("no service with name possum"))
		})
	})

	Context("when a possum service exists", func() {
		Context("and password is not present", func() {
			BeforeEach(func() {
				vcapServicesJSON = `{
"user-provided": [
 {
  "credentials": {
  },
  "label": "user-provided",
  "name": "possum",
  "syslog_drain_url": "",
  "tags": []
 }
]
}`
			})

			It("return an empty string", func() {
				password, err := utils.GetPassword()
				Ω(err).Should(BeNil())
				Ω(password).Should(BeEmpty())
			})
		})

		Context("and password is present", func() {
			BeforeEach(func() {
				vcapServicesJSON = `{
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
}`
			})

			It("Returns the configured password", func() {
				password, err := utils.GetPassword()
				Ω(err).Should(BeNil())
				Ω(password).Should(Equal("admin"))
			})
		})

		Context("When unmarshalling raises an error", func() {
			BeforeEach(func() {
				vcapServicesJSON = `{
  "user-provided": [
   {
    "credenti
      "key":"value"
    },
    "label": "user-provided",
    "name": "possum-db",
    "syslog_drain_url": "",
    "tags": []
   }
  ]
 }`
			})

			It("Returns an error", func() {
				_, err := utils.GetPassword()
				Ω(err).Should(MatchError("invalid character '\\n' in string literal"))
			})
		})
	})
})
