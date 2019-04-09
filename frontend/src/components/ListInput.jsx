import React, { Component } from 'react';
import PropTypes from 'prop-types';
import autobind from 'autobind-decorator';
import CreatableSelect from 'react-select/lib/Creatable';

import 'components/ListInput.scss';

const createOption = label => {
  return {
    label,
    value: label,
  };
};

const toOptionList = value => {
  return value ? value.map(createOption) : [];
};

const toValueList = list => {
  return list.map(obj => obj.value);
};

class ListInput extends Component {
  constructor(props) {
    super(props);
    this.state = {
      inputValue: ''
    };
  }

  @autobind
  onChange(options) {
    if (this.props.onChange) {
      this.props.onChange(toValueList(options));
    }
  }

  hasValue(value) {
    return this.props.value.includes(value);
  }

  @autobind
  onKeyDown(event) {
    const { inputValue } = this.state;
    const options = toOptionList(this.props.value);
    if (!inputValue) return;
    switch (event.key) {
      case 'Enter':
      case 'Tab':
      case ',':
      case ';':
      case ' ': {
        this.setState({ inputValue: '' });
        if (!this.hasValue(inputValue)) {
          this.onChange([...options, createOption(inputValue)]);
        }
        event.preventDefault();
      }
    }
  }

  @autobind
  onInputChange(inputValue) {
    this.setState({ inputValue });
  }

  render() {
    const options = toOptionList(this.props.value);
    return (
      <CreatableSelect
        className="ListInput Select"
        classNamePrefix="Select"
        components={{
          DropdownIndicator: null
        }}
        isClearable={this.props.isClearable}
        isMulti
        menuIsOpen={false}
        placeholder={this.props.placeholder}
        onChange={this.onChange}
        onKeyDown={this.onKeyDown}
        onInputChange={this.onInputChange}
        inputValue={this.state.inputValue}
        value={options}
        inputId={this.props.id || `select-${Date.now()}-${Math.floor(Math.random()*1000)}`}
      />
    );
  }
}

ListInput.defaultProps = {
  isClearable: false,
  placeholder: 'Type your list...',
  value: []
};

ListInput.propTypes = {
  id: PropTypes.string,
  placeholder: PropTypes.string,
  onChange: PropTypes.func,
  isClearable: PropTypes.bool,
  value: PropTypes.arrayOf(PropTypes.string)
};

export default ListInput;
