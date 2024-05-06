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