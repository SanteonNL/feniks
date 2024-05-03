SELECT
    json_build_object(
        'id',
        practitioner.practitioner_id,
        'name',
        (
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
            WHERE
                humanName.identificatienummer = practitioner.practitioner_id
        )
    ) AS practitioner_json
FROM
    practitioner practitioner;