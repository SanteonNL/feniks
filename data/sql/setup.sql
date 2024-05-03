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
    name_use character varying(255)
);

INSERT INTO public.names (identificatienummer, firstname, lastname, name_use)
VALUES ('123', 'John', 'Doe', 'Official');

INSERT INTO public.names (identificatienummer, firstname, lastname, name_use)
VALUES ('123', '', 'Smith', 'Alternate');

INSERT INTO public.names (identificatienummer, firstname, lastname, name_use)
VALUES ('987', 'Alice', 'Johnson', 'Official');

INSERT INTO public.names (identificatienummer, firstname, lastname, name_use)
VALUES ('987', 'Bob', 'Williams', 'Alternate');

INSERT INTO public.names (identificatienummer, firstname, lastname, name_use)
VALUES ('456', 'Michael', 'Brown', 'Official');

INSERT INTO public.names (identificatienummer, firstname, lastname, name_use)
VALUES ('456', 'Emily', 'Davis', 'Alternate');

INSERT INTO public.names (identificatienummer, firstname, lastname, name_use)
VALUES ('789', 'David', 'Miller', 'Official');

INSERT INTO public.names (identificatienummer, firstname, lastname, name_use)
VALUES ('P002', 'Olivia', 'Wilson', 'Alternate');

INSERT INTO public.names (identificatienummer, firstname, lastname, name_use)
VALUES ('P001', 'Daniel', 'Anderson', 'Official');

INSERT INTO public.names (identificatienummer, firstname, lastname, name_use)
VALUES ('P001', 'Sophia', 'Taylor', 'Alternate');

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
