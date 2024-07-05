SELECT
    p.identificatienummer AS id,
    humanName.lastname as family,
    string_agg(humanName.firstname, '||') AS name
FROM
    patient p
    JOIN names humanName ON humanName.identificatienummer = p.identificatienummer
WHERE 1=1
--AND p.identificatienummer = :id    
GROUP BY
    p.identificatienummer, humanName.lastname;