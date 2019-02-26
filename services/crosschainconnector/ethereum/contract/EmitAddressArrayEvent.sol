pragma solidity ^0.5.0;
contract EmitAddressArrayEvent {
    event EventWithAddressArray(address[] value);

    function fire(address[] memory value) public {
        emit EventWithAddressArray(value);
    }
}
