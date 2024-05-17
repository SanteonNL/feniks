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
            (SELECT
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
            FROM
                names humanName