SELECT
    json_build_object(
        'id',
        practitioner.practitioner_id,
        'name',
        (
            -- humanName_.sql
            WHERE
                humanName.identificatienummer = practitioner.practitioner_id
        )
    ) AS practitioner_json
FROM
    practitioner practitioner;