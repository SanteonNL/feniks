SELECT json_agg(
        json_build_object(
            'use',
            'official',
            'given',
            json_build_array(n.firstname),
            'family',
            n.lastname
        )
    )
FROM patient_names n