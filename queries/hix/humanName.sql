SELECT
    json_agg(
        json_build_object(
            'use',
            humanName.name_use,
            'given',
            json_build_array(humanName.firstname, 'fixed secondName'),
            'family',
            humanName.lastname
        )
    )
FROM
    names humanName