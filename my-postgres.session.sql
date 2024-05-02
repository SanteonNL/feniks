        CREATE TABLE patient_hix_patient (
            identificatienummer VARCHAR(13) PRIMARY KEY,
            name VARCHAR(255),

        );

        CREATE TABLE patient_names (
            identificatienummer VARCHAR(13),
            firstName VARCHAR(255),
            lastName VARCHAR(255),
            name VARCHAR(255)
            -- FOREIGN KEY (identificatienummer) REFERENCES patient_hix_patient(identificatienummer)
        );

        INSERT INTO patient_names (identificatienummer, firstName, lastName, name)
        VALUES ('456', 'John', 'Doe', 'John Doe');

        INSERT INTO patient_names (identificatienummer, firstName, lastName, name)
        VALUES ('789', 'Jane', 'Smith', 'Jane Smith');

        -- Additional examples for patient 456
        INSERT INTO patient_names (identificatienummer, firstName, lastName, name)
        VALUES ('456', 'Michael', 'Johnson', 'Michael Johnson');

        INSERT INTO patient_names (identificatienummer, firstName, lastName, name)
        VALUES ('456', 'Sarah', 'Williams', 'Sarah Williams');

        -- Additional examples for patient 789
        INSERT INTO patient_names (identificatienummer, firstName, lastName, name)
        VALUES ('789', 'David', 'Brown', 'David Brown');

        INSERT INTO patient_names (identificatienummer, firstName, lastName, name)
        VALUES ('789', 'Emily', 'Davis', 'Emily Davis');


