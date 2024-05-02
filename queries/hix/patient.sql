SELECT
    json_build_object(
        'resourceType',
        'Patient',
        'id',
        p.identificatienummer,
        'gender', -- "http://hl7.org/fhir/ValueSet/administrative-gender"
        CASE
            WHEN p.geslachtcode = 'M' THEN 'male'
            WHEN p.geslachtcode = 'F' THEN  'female'
            ELSE 'unknown'
        END, 
        'name',
        (
            SELECT
                json_agg(
                    json_build_object(
                        'use',
                        'official',
                        'given',
                        json_build_array(n.firstname, 'fixed secondName'),
                        'family',
                        n.lastname
                    )
                )
            FROM
                patient_names n
            WHERE
                n.identificatienummer = p.identificatienummer
        ),
        'birthDate',
        p.geboortedatum,
        'deceasedDateTime',
        p.datumoverlijden
    ) AS json_output
FROM
    patient_hix_patient p;