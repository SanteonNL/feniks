SELECT
    json_agg(
        json_build_object(
            'use',
            humanName.name_use,
            'given',
            json_build_array(humanName.firstname, 'fixed secondName'),
            'family',
            humanName.lastname,
            'period',
            (
                SELECT
                    json_build_object(
                        'reference', 'http://example.com/fhir/Period/',
                        'display', 'Period display'
                    )
                FROM
                    (SELECT 1) AS dummy_table
            )
        )
    )
FROM
    names humanName