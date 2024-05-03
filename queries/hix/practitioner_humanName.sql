SELECT
    json_agg(
        json_build_object(
            'use',
            humanName.name_use,
            'given',
            json_build_array(humanName.firstname, 'PractionersSecondName'),
            'family',
            humanName.lastname
        )
    )
FROM
    names humanName