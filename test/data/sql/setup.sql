CREATE TABLE public.patient (
    identificatienummer character varying(13),
    geslachtcode character varying(255),
    geslachtomschrijving character varying(255),
    gerelateerdpersoonid character varying(20),
    gerelateerderelatie character varying(255),
    land character varying(100),
    geboortedatum timestamp without time zone,
    datumoverlijden date,
    datumcheckstatusoverlijden character varying(255)
);

INSERT INTO public.patient (identificatienummer, geslachtcode, geslachtomschrijving, gerelateerdpersoonid, gerelateerderelatie, land, geboortedatum, datumoverlijden, datumcheckstatusoverlijden)
VALUES ('123', 'M', 'Male', '987654321', '123456', 'USA', '1990-01-01', '2022-05-10', 'Checked');

INSERT INTO public.patient (identificatienummer, geslachtcode, geslachtomschrijving, gerelateerdpersoonid, gerelateerderelatie, land, geboortedatum, datumoverlijden, datumcheckstatusoverlijden)
VALUES ('987', 'F', 'Female', '123456789', '654321', 'UK', '1985-05-20', NULL, 'Unchecked');

INSERT INTO public.patient (identificatienummer, geslachtcode, geslachtomschrijving, gerelateerdpersoonid, gerelateerderelatie, land, geboortedatum, datumoverlijden, datumcheckstatusoverlijden)
VALUES ('456', 'M', 'Male', '789012345', '789', 'Canada', '1978-12-10', NULL, 'Unchecked');

INSERT INTO public.patient (identificatienummer, geslachtcode, geslachtomschrijving, gerelateerdpersoonid, gerelateerderelatie, land, geboortedatum, datumoverlijden, datumcheckstatusoverlijden)
VALUES ('789', 'F', 'Female', '456789012', '567890', 'Australia', '1995-08-15', NULL, 'Unchecked');

INSERT INTO public.patient (identificatienummer, geslachtcode, geslachtomschrijving, gerelateerdpersoonid, gerelateerderelatie, land, geboortedatum, datumoverlijden, datumcheckstatusoverlijden)
VALUES ('234', 'M', 'Male', '890123456', '901234', 'Germany', '1980-03-25', '2021-11-30', 'Checked');

CREATE TABLE public.names (
    identificatienummer character varying(13),
    firstname character varying(255),
    lastname character varying(255),
    name_use character varying(255),
    period_start date,
    period_end date
);

INSERT INTO public.names (identificatienummer, firstname, lastname, name_use, period_start, period_end)
VALUES ('123', 'John', 'Doe', 'Official', '2022-01-01', '2022-12-31');

INSERT INTO public.names (identificatienummer, firstname, lastname, name_use, period_start, period_end)
VALUES ('987', 'Alice', 'Johnson', 'Official', '2022-01-01', '2022-12-31');

INSERT INTO public.names (identificatienummer, firstname, lastname, name_use, period_start, period_end)
VALUES ('987', 'Bob', 'Williams', 'Alternate', '2022-01-01', '2022-12-31');

INSERT INTO public.names (identificatienummer, firstname, lastname, name_use, period_start, period_end)
VALUES ('456', 'Michael', 'Brown', 'Official', '2022-01-01', '2022-12-31');

INSERT INTO public.names (identificatienummer, firstname, lastname, name_use, period_start, period_end)
VALUES ('456', 'Emily', 'Davis', 'Alternate', '2022-01-01', '2022-12-31');

INSERT INTO public.names (identificatienummer, firstname, lastname, name_use, period_start, period_end)
VALUES ('789', 'David', 'Miller', 'Official', '2022-01-01', '2022-12-31');

INSERT INTO public.names (identificatienummer, firstname, lastname, name_use, period_start, period_end)
VALUES ('P002', 'Olivia', 'Wilson', 'Alternate', '2022-01-01', '2022-12-31');

INSERT INTO public.names (identificatienummer, firstname, lastname, name_use, period_start, period_end)
VALUES ('P001', 'Daniel', 'Anderson', 'Official', '2022-01-01', '2022-12-31');

INSERT INTO public.names (identificatienummer, firstname, lastname, name_use, period_start, period_end)
VALUES ('P001', 'Sophia', 'Taylor', 'Alternate', '2022-01-01', '2022-12-31');
CREATE TABLE public.practitioner (
    practitioner_id character varying(20),
    practitioner_name character varying(255)
);

INSERT INTO public.practitioner (practitioner_id, practitioner_name)
VALUES ('P001', 'Dr. Smith');

INSERT INTO public.practitioner (practitioner_id, practitioner_name)
VALUES ('P002', 'Dr. Johnson');

CREATE TABLE public.patient_practitioner (
    identificatienummer character varying(13),
    practitioner_id character varying(20)
);

INSERT INTO public.patient_practitioner (identificatienummer, practitioner_id)
VALUES (RIGHT('123', 3), 'P001');

INSERT INTO public.patient_practitioner (identificatienummer, practitioner_id)
VALUES (RIGHT('987', 3), 'P001a');

INSERT INTO public.patient_practitioner (identificatienummer, practitioner_id)
VALUES (RIGHT('456', 3), 'P002');

INSERT INTO public.patient_practitioner (identificatienummer, practitioner_id)
VALUES (RIGHT('789', 3), 'P002');

INSERT INTO public.patient_practitioner (identificatienummer, practitioner_id)
VALUES (RIGHT('234', 3), 'P002');


INSERT INTO public.names (identificatienummer, firstname, lastname, name_use)
VALUES (RIGHT('123', 3), 'Mark', 'Johnson', 'Official');

INSERT INTO public.names (identificatienummer, firstname, lastname, name_use)
VALUES (RIGHT('123', 3), 'Sarah', 'Smith', 'Alternate');

INSERT INTO public.names (identificatienummer, firstname, lastname, name_use)
VALUES (RIGHT('987', 3), 'Emily', 'Brown', 'Official');

INSERT INTO public.names (identificatienummer, firstname, lastname, name_use)
VALUES (RIGHT('987', 3), 'Jacob', 'Davis', 'Alternate');

INSERT INTO public.names (identificatienummer, firstname, lastname, name_use)
VALUES (RIGHT('456', 3), 'Emma', 'Miller', 'Official');

INSERT INTO public.names (identificatienummer, firstname, lastname, name_use)
VALUES (RIGHT('456', 3), 'Noah', 'Wilson', 'Alternate');

INSERT INTO public.names (identificatienummer, firstname, lastname, name_use)
VALUES (RIGHT('789', 3), 'Liam', 'Anderson', 'Official');

INSERT INTO public.names (identificatienummer, firstname, lastname, name_use)
VALUES (RIGHT('789', 3), 'Ava', 'Taylor', 'Alternate');

INSERT INTO public.names (identificatienummer, firstname, lastname, name_use)
VALUES (RIGHT('234', 3), 'Mia', 'Anderson', 'Official');

INSERT INTO public.names (identificatienummer, firstname, lastname, name_use)
VALUES (RIGHT('234', 3), 'Ethan', 'Taylor', 'Alternate');

-- Create new mothers for existing patients
INSERT INTO public.patient (identificatienummer, geslachtcode, geslachtomschrijving, gerelateerdpersoonid, gerelateerderelatie, land, geboortedatum, datumoverlijden, datumcheckstatusoverlijden)
VALUES ('123M', 'F', 'Female', '123', 'Mother', 'USA', '1970-01-01', NULL, 'Unchecked');

INSERT INTO public.patient (identificatienummer, geslachtcode, geslachtomschrijving, gerelateerdpersoonid, gerelateerderelatie, land, geboortedatum, datumoverlijden, datumcheckstatusoverlijden)
VALUES ('987M', 'F', 'Female', '987', 'Mother', 'UK', '1965-05-20', NULL, 'Unchecked');

INSERT INTO public.patient (identificatienummer, geslachtcode, geslachtomschrijving, gerelateerdpersoonid, gerelateerderelatie, land, geboortedatum, datumoverlijden, datumcheckstatusoverlijden)
VALUES ('456M', 'F', 'Female', '456', 'Mother', 'Canada', '1958-12-10', NULL, 'Unchecked');

INSERT INTO public.patient (identificatienummer, geslachtcode, geslachtomschrijving, gerelateerdpersoonid, gerelateerderelatie, land, geboortedatum, datumoverlijden, datumcheckstatusoverlijden)
VALUES ('789M', 'F', 'Female', '789', 'Mother', 'Australia', '1985-08-15', NULL, 'Unchecked');

INSERT INTO public.patient (identificatienummer, geslachtcode, geslachtomschrijving, gerelateerdpersoonid, gerelateerderelatie, land, geboortedatum, datumoverlijden, datumcheckstatusoverlijden)
VALUES ('234M', 'F', 'Female', '234', 'Mother', 'Germany', '1960-03-25', NULL, 'Unchecked');

-- Create a new couple table that relates a patientid to its mother
CREATE TABLE public.couple (
    patient_id character varying(13),
    mother_id character varying(13)
);

INSERT INTO public.couple (patient_id, mother_id)
VALUES ('123', '123M');

INSERT INTO public.couple (patient_id, mother_id)
VALUES ('987', '987M');

INSERT INTO public.couple (patient_id, mother_id)
VALUES ('456', '456M');

INSERT INTO public.couple (patient_id, mother_id)
VALUES ('789', '789M');

INSERT INTO public.couple (patient_id, mother_id)
VALUES ('234', '234M');


CREATE TABLE contacts (
    id VARCHAR(50) PRIMARY KEY,
    name VARCHAR(100),
    relationship VARCHAR(50),
    gender VARCHAR(10),
    organization VARCHAR(100),
    patient_id VARCHAR(50)
);

INSERT INTO contacts (id, name, relationship, gender, organization, patient_id)
VALUES ('456', 'John Doe', 'Friend', 'male', 'Hospital A', '123');

INSERT INTO contacts (id, name, relationship, gender, organization, patient_id)
VALUES ('789', 'Jane Smith', 'Family', 'female', 'Hospital B', '123');


CREATE TABLE contact_points (
    id SERIAL PRIMARY KEY,
    contact_id VARCHAR(50),
    system VARCHAR(50),
    value VARCHAR(100),
    use VARCHAR(50),
    FOREIGN KEY (contact_id) REFERENCES contacts(id)
);

INSERT INTO contact_points (contact_id, system, value, use)
VALUES ('456', 'phone', '+1234567890', 'home');

INSERT INTO contact_points (contact_id, system, value, use)
VALUES ('456', 'email', 'john.doe@example.com', 'work');

INSERT INTO contact_points (contact_id, system, value, use)
VALUES ('789', 'phone', '+9876543210', 'mobile');

