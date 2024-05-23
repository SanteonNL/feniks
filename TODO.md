- pass search parameter to GetAllPatients( searchParam)

func main() {

	// configPath := util.GetAbsolutePath("config/connections.json")

	// file, err := os.Open(configPath)
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// defer file.Close()

	// var config Config
	// if err := json.NewDecoder(file).Decode(&config); err != nil {
	// 	log.Fatal(err)
	// }

	// app := &Application{
	// 	Services: []Service{},
	// }

	// for _, serviceConfig := range config.Services {
	// 	service, err := NewService(serviceConfig)
	// 	if err != nil {
	// 		log.Fatal(err)
	// 	}
	// 	app.Services = append(app.Services, service)
	// }

	r := mux.NewRouter()
	// r.HandleFunc("/patient/{id}", app.GetPatient).Methods("GET")
	// r.HandleFunc("/patients/{id}", app.GetAllPatients).Methods("GET")
	r.HandleFunc("/patients2", GetAllPatients2).Methods("GET")
	log.Fatal(http.ListenAndServe(":8080", r))

}