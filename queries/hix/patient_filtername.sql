SELECT
    json_build_object(
        'resourceType',
        'Patient',
        'id',
        p.identificatienummer,
        'gender',
        -- targetsystem: https://www.hl7.org/fhir/valueset/gender
        CASE
            WHEN p.geslachtcode = 'M' THEN 'male'
            WHEN p.geslachtcode = 'F' THEN 'female'
            ELSE 'unknown'
        END,

        -- patient_humaName.sql
        'name',
        (
            SELECT
                json_agg(
                    json_build_object(
                        'use',
                        'official' , -- targetsysem: http://hl7.org/fhir/ValueSet/name-use
                        'given',
                        json_build_array(humanName.firstname, 'fixed secondName'),
                        'family',
                        humanName.lastname,
                        'period',
                        (
                            SELECT
                                json_build_object(
                                    'start',
                                    'beginjanuari',
                                    'end',
                                    'jaar binnen 10 jaar'
                                )
                            FROM
                                (
                                    SELECT
                                        1
                                ) AS dummy_table
                        )
                    )
                )
            FROM
                names humanName
            WHERE
                humanName.identificatienummer = p.identificatienummer
        ),
        'birthDate',
        p.geboortedatum,
        'deceasedDateTime',
        p.datumoverlijden,
        'generalPractitioner',
        (
            SELECT
                json_agg(
                    json_build_object(
                        'reference',
                        CONCAT('Practitioner/', pr.practitioner_id),
                        'type',
                        'Practitioner'
                    )
                )
            FROM
                practitioner pr
                INNER JOIN patient_practitioner pp ON pr.practitioner_id = pp.practitioner_id
            WHERE
                pp.identificatienummer = p.identificatienummer
        )
    ) AS json_output
FROM
    patient p;