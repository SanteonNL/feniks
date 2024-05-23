SELECT
    json_build_object(
        'resourceType',
        'Patient',
        'id',
        p.identificatienummer,
        'gender',
        CASE
            WHEN p.geslachtcode = 'M' THEN 'male'
            WHEN p.geslachtcode = 'F' THEN 'female'
            ELSE 'unknown'
        END,
        'name',
        (
            SELECT
                json_agg(
                    json_build_object(
                        'use',
                        'official' ,
                        'given',
                        json_build_array(humanName.firstname, 'fixed secondName'),
                        'family',
                        humanName.lastname,
                        'period',
                        (
                            SELECT
                                json_build_object(
                                    'start',
                                    '1995-08-15',
                                    'end',
                                    '1999-08-15'
                                )
                            FROM
                                (
                                    SELECT
                                        1
                                ) AS period
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
                        CONCAT('Practitioner/', pr.practitioner_id)
                    )
                )
            FROM
                practitioner pr
                INNER JOIN patient_practitioner pp ON pr.practitioner_id = pp.practitioner_id
            WHERE
                pp.identificatienummer = p.identificatienummer
        ),
        'link', 
        (
            SELECT
                json_agg(
                    json_build_object(
                        'other',
                        CONCAT('Patient/', cp.mother_id),
                        'type',
                        'seealso'
                    )
                )
            FROM
                couple cp
            WHERE
                cp.patient_id = p.identificatienummer
        )
        
    ) AS json_output
FROM
    patient p;